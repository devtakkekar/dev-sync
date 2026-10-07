package syncengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/devsync/dev-sync/internal/config"
	"github.com/devsync/dev-sync/internal/discord"
	"github.com/devsync/dev-sync/internal/executor"
	"github.com/devsync/dev-sync/internal/ignore"
	"github.com/devsync/dev-sync/internal/logging"
	"github.com/devsync/dev-sync/internal/planner"
	"github.com/devsync/dev-sync/internal/safety"
	"github.com/devsync/dev-sync/internal/scanner"
	"github.com/devsync/dev-sync/internal/state"
)

// EngineSyncOptions configures a sync run.
type EngineSyncOptions struct {
	ReplicaID    string // Empty for all enabled replicas
	DryRun       bool
	Force        bool
	DeepCompare  bool
	Verbose      bool
	OnProgress   func(replicaID string, item *planner.PlanItem, err error)
}

// SyncEngine coordinates the scanning, planning, execution, state updates, and notifications.
type SyncEngine struct {
	ProjectRoot string
	Config      *config.Config
	State       *state.State
	Matcher     *ignore.IgnoreMatcher
	Discord     *discord.Client
}

// NewSyncEngine initializes a SyncEngine for the given project directory.
func NewSyncEngine(projectRoot string) (*SyncEngine, error) {
	cfg, err := config.LoadConfig(projectRoot)
	if err != nil {
		return nil, err
	}

	st, err := state.LoadState(projectRoot)
	if err != nil {
		logging.Warn("State file warning: %v (recovering clean state)", err)
		st = state.NewState()
	}

	ignoreFile := filepath.Join(projectRoot, config.IgnoreFileName)
	matcher, err := ignore.LoadIgnoreFile(ignoreFile)
	if err != nil {
		logging.Warn("Failed to load .devsyncignore: %v", err)
		matcher = ignore.NewMatcher()
	}

	discordClient := discord.NewClient(cfg)

	return &SyncEngine{
		ProjectRoot: projectRoot,
		Config:      cfg,
		State:       st,
		Matcher:     matcher,
		Discord:     discordClient,
	}, nil
}

// Sync performs synchronization for either a single target replica or all enabled replicas.
func (e *SyncEngine) Sync(ctx context.Context, opts EngineSyncOptions) ([]*executor.ExecuteResult, error) {
	var targetReplicas []config.ReplicaConfig

	if opts.ReplicaID != "" {
		rep, _ := e.Config.FindReplicaByID(opts.ReplicaID)
		if rep == nil {
			return nil, fmt.Errorf("replica with ID '%s' not found in configuration", opts.ReplicaID)
		}
		targetReplicas = append(targetReplicas, *rep)
	} else {
		for _, r := range e.Config.Replicas {
			if r.Enabled {
				targetReplicas = append(targetReplicas, r)
			}
		}
	}

	if len(targetReplicas) == 0 {
		return nil, fmt.Errorf("no enabled replicas configured to synchronize")
	}

	// Scan source directory once for all replicas
	logging.Debug("Scanning source directory: %s", e.Config.Source)
	sourceScan, err := scanner.ScanDirectory(
		e.Config.Source,
		e.Matcher,
		e.Config.Filesystem.SymlinkMode,
		opts.DeepCompare,
	)
	if err != nil {
		return nil, fmt.Errorf("source scan failed: %w", err)
	}

	results := make([]*executor.ExecuteResult, len(targetReplicas))
	var wg sync.WaitGroup
	// Limit concurrency to max 4 replicas at a time
	sem := make(chan struct{}, 4)

	var hasErrors bool

	for i, replica := range targetReplicas {
		wg.Add(1)
		go func(idx int, rep config.ReplicaConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := e.syncSingleReplica(ctx, rep, sourceScan, opts)
			results[idx] = res
			if !res.Success {
				hasErrors = true
			}
		}(i, replica)
	}

	wg.Wait()

	// Update and save state (if not dry run)
	if !opts.DryRun {
		e.State.LastSync = time.Now()
		for _, r := range results {
			if r == nil {
				continue
			}
			repState, ok := e.State.Replicas[r.ReplicaID]
			if !ok {
				repState = &state.ReplicaState{}
				e.State.Replicas[r.ReplicaID] = repState
			}
			if r.Success {
				repState.Status = "in_sync"
				repState.LastSuccessfulSync = time.Now()
				repState.FilesCount = len(sourceScan.Files)
				repState.BytesCount = sourceScan.TotalBytes
				repState.LastError = ""
			} else {
				repState.Status = "error"
				if len(r.Errors) > 0 {
					repState.LastError = r.Errors[0].Err
				}
			}
		}
		_ = state.SaveState(e.ProjectRoot, e.State)
	}

	// Discord notification
	if !opts.DryRun {
		totalCreated := 0
		totalModified := 0
		totalDeleted := 0
		var successfulReps []string
		var maxDuration time.Duration

		for _, r := range results {
			if r == nil {
				continue
			}
			totalCreated += r.Created
			totalModified += r.Modified
			totalDeleted += r.Deleted
			if r.Duration > maxDuration {
				maxDuration = r.Duration
			}
			if r.Success {
				successfulReps = append(successfulReps, r.ReplicaID)
			} else {
				errMsg := "unknown error"
				if len(r.Errors) > 0 {
					errMsg = r.Errors[0].Err
				}
				e.Discord.NotifySyncFailed(ctx, filepath.Base(e.Config.Source), r.ReplicaID, errMsg)
			}
		}

		if len(successfulReps) > 0 && (totalCreated > 0 || totalModified > 0 || totalDeleted > 0) {
			e.Discord.NotifySyncCompleted(
				ctx,
				filepath.Base(e.Config.Source),
				successfulReps,
				totalCreated,
				totalModified,
				totalDeleted,
				maxDuration,
			)
		}
	}

	if hasErrors {
		return results, fmt.Errorf("one or more replicas failed during synchronization")
	}

	return results, nil
}

func (e *SyncEngine) syncSingleReplica(ctx context.Context, rep config.ReplicaConfig, sourceScan *scanner.ScanResult, opts EngineSyncOptions) *executor.ExecuteResult {
	start := time.Now()
	res := &executor.ExecuteResult{
		ReplicaID:   rep.ID,
		ReplicaPath: rep.Path,
		Success:     false,
	}

	// 1. Verify replica directory accessibility
	fi, err := os.Stat(rep.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// Replica directory doesn't exist yet, we will create it during execute
			logging.Debug("Replica %s directory does not exist, will create", rep.ID)
		} else {
			res.Errors = append(res.Errors, executor.ItemError{
				RelPath: ".",
				Action:  "ACCESS",
				Err:     fmt.Sprintf("cannot access replica path: %v", err),
			})
			res.Duration = time.Since(start)
			return res
		}
	} else if !fi.IsDir() {
		res.Errors = append(res.Errors, executor.ItemError{
			RelPath: ".",
			Action:  "TYPE",
			Err:     "replica path exists but is not a directory",
		})
		res.Duration = time.Since(start)
		return res
	}

	// 2. Scan replica
	repScan := &scanner.ScanResult{
		Files:       make(map[string]*scanner.FileInfo),
		Directories: []string{},
	}
	if fi != nil && fi.IsDir() {
		repScan, err = scanner.ScanDirectory(
			rep.Path,
			e.Matcher,
			e.Config.Filesystem.SymlinkMode,
			opts.DeepCompare,
		)
		if err != nil {
			res.Errors = append(res.Errors, executor.ItemError{
				RelPath: ".",
				Action:  "SCAN",
				Err:     fmt.Sprintf("failed to scan replica: %v", err),
			})
			res.Duration = time.Since(start)
			return res
		}
	}

	// 3. Build Plan
	plan, err := planner.BuildPlan(rep.ID, rep.Path, sourceScan, repScan, planner.PlanOptions{
		DeleteExtraneous: e.Config.Sync.DeleteExtraneous,
		DeepCompare:      opts.DeepCompare,
		BufferSizeKB:     e.Config.Filesystem.BufferSizeKB,
	})
	if err != nil {
		res.Errors = append(res.Errors, executor.ItemError{
			RelPath: ".",
			Action:  "PLAN",
			Err:     fmt.Sprintf("failed to build plan: %v", err),
		})
		res.Duration = time.Since(start)
		return res
	}

	// 4. Execute Plan
	execOpts := executor.ExecuteOptions{
		DryRun:              opts.DryRun,
		BufferSizeKB:        e.Config.Filesystem.BufferSizeKB,
		PreserveTimestamps:  e.Config.Sync.PreserveTimestamps,
		PreservePermissions: e.Config.Sync.PreservePermissions,
		VerifyAfterSync:     e.Config.Sync.VerifyAfterSync,
		OnProgress: func(item *planner.PlanItem, itemErr error) {
			if opts.OnProgress != nil {
				opts.OnProgress(rep.ID, item, itemErr)
			}
		},
	}

	execRes, execErr := executor.Execute(ctx, e.Config.Source, plan, execOpts)
	if execErr != nil {
		res.Errors = append(res.Errors, executor.ItemError{
			RelPath: ".",
			Action:  "EXECUTE",
			Err:     execErr.Error(),
		})
		res.Duration = time.Since(start)
		return res
	}

	return execRes
}
