package executor

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/devsync/dev-sync/internal/filesystem"
	"github.com/devsync/dev-sync/internal/planner"
	"github.com/devsync/dev-sync/internal/safety"
)

// ItemError records an error that occurred while processing a specific path.
type ItemError struct {
	RelPath string `json:"relPath"`
	Action  string `json:"action"`
	Err     string `json:"error"`
}

// ExecuteOptions configures plan execution.
type ExecuteOptions struct {
	DryRun              bool
	BufferSizeKB        int
	PreserveTimestamps  bool
	PreservePermissions bool
	VerifyAfterSync     bool
	OnProgress          func(item *planner.PlanItem, err error)
}

// ExecuteResult records the outcome of a replica synchronization.
type ExecuteResult struct {
	ReplicaID        string        `json:"replicaId"`
	ReplicaPath      string        `json:"replicaPath"`
	Success          bool          `json:"success"`
	Created          int           `json:"created"`
	Modified         int           `json:"modified"`
	Deleted          int           `json:"deleted"`
	Unchanged        int           `json:"unchanged"`
	BytesTransferred int64         `json:"bytesTransferred"`
	Duration         time.Duration `json:"duration"`
	Errors           []ItemError   `json:"errors,omitempty"`
}

// Execute runs the synchronization plan against the replica directory.
func Execute(ctx context.Context, sourceRoot string, plan *planner.SyncPlan, opts ExecuteOptions) (*ExecuteResult, error) {
	start := time.Now()
	res := &ExecuteResult{
		ReplicaID:   plan.ReplicaID,
		ReplicaPath: plan.ReplicaPath,
		Success:     true,
		Unchanged:   plan.Counts[planner.ActionUnchanged],
	}

	normReplica, err := safety.NormalizePath(plan.ReplicaPath)
	if err != nil {
		res.Success = false
		return res, fmt.Errorf("invalid replica root: %w", err)
	}

	if opts.DryRun {
		res.Created = plan.Counts[planner.ActionCreate]
		res.Modified = plan.Counts[planner.ActionUpdate]
		res.Deleted = plan.Counts[planner.ActionDelete]
		res.BytesTransferred = plan.TotalBytesSync
		res.Duration = time.Since(start)
		return res, nil
	}

	// 1. Ensure root replica directory exists
	if err := filesystem.EnsureDir(normReplica); err != nil {
		res.Success = false
		return res, fmt.Errorf("failed to create replica root directory %s: %w", normReplica, err)
	}

	// 2. Pre-create directories
	for _, dirRel := range plan.DirsToCreate {
		select {
		case <-ctx.Done():
			res.Success = false
			return res, ctx.Err()
		default:
		}

		targetDir, err := safety.SafeResolvePath(normReplica, dirRel)
		if err != nil {
			res.Errors = append(res.Errors, ItemError{
				RelPath: dirRel,
				Action:  "MKDIR",
				Err:     err.Error(),
			})
			res.Success = false
			continue
		}
		if err := filesystem.EnsureDir(targetDir); err != nil {
			res.Errors = append(res.Errors, ItemError{
				RelPath: dirRel,
				Action:  "MKDIR",
				Err:     err.Error(),
			})
			res.Success = false
		}
	}

	// 3. Process file actions
	for _, item := range plan.Items {
		select {
		case <-ctx.Done():
			res.Success = false
			return res, ctx.Err()
		default:
		}

		switch item.Action {
		case planner.ActionUnchanged:
			continue

		case planner.ActionCreate, planner.ActionUpdate:
			destPath, err := safety.SafeResolvePath(normReplica, item.RelPath)
			if err != nil {
				res.Errors = append(res.Errors, ItemError{
					RelPath: item.RelPath,
					Action:  string(item.Action),
					Err:     err.Error(),
				})
				res.Success = false
				if opts.OnProgress != nil {
					opts.OnProgress(item, err)
				}
				continue
			}

			// Atomic copy
			copyErr := filesystem.AtomicCopyFile(
				item.SourceInfo.FullPath,
				destPath,
				opts.BufferSizeKB,
				opts.PreserveTimestamps,
				opts.PreservePermissions,
			)

			if copyErr != nil {
				res.Errors = append(res.Errors, ItemError{
					RelPath: item.RelPath,
					Action:  string(item.Action),
					Err:     copyErr.Error(),
				})
				res.Success = false
				if opts.OnProgress != nil {
					opts.OnProgress(item, copyErr)
				}
				continue
			}

			// Verify after sync if requested
			if opts.VerifyAfterSync {
				repHash, err := filesystem.ComputeFileHash(destPath, opts.BufferSizeKB)
				if err != nil {
					res.Errors = append(res.Errors, ItemError{
						RelPath: item.RelPath,
						Action:  "VERIFY",
						Err:     fmt.Sprintf("verification failed: %v", err),
					})
					res.Success = false
					continue
				}

				srcHash := item.SourceInfo.Hash
				if srcHash == "" {
					srcHash, _ = filesystem.ComputeFileHash(item.SourceInfo.FullPath, opts.BufferSizeKB)
				}

				if repHash != srcHash {
					res.Errors = append(res.Errors, ItemError{
						RelPath: item.RelPath,
						Action:  "VERIFY",
						Err:     fmt.Sprintf("hash mismatch after sync: source=%s, replica=%s", srcHash, repHash),
					})
					res.Success = false
					continue
				}
			}

			if item.Action == planner.ActionCreate {
				res.Created++
			} else {
				res.Modified++
			}
			res.BytesTransferred += item.SourceInfo.Size
			if opts.OnProgress != nil {
				opts.OnProgress(item, nil)
			}

		case planner.ActionDelete:
			destPath, err := safety.SafeResolvePath(normReplica, item.RelPath)
			if err != nil {
				res.Errors = append(res.Errors, ItemError{
					RelPath: item.RelPath,
					Action:  string(item.Action),
					Err:     err.Error(),
				})
				res.Success = false
				if opts.OnProgress != nil {
					opts.OnProgress(item, err)
				}
				continue
			}

			delErr := filesystem.SafeRemoveFile(normReplica, destPath)
			if delErr != nil {
				res.Errors = append(res.Errors, ItemError{
					RelPath: item.RelPath,
					Action:  string(item.Action),
					Err:     delErr.Error(),
				})
				res.Success = false
				if opts.OnProgress != nil {
					opts.OnProgress(item, delErr)
				}
				continue
			}

			res.Deleted++
			if opts.OnProgress != nil {
				opts.OnProgress(item, nil)
			}
		}
	}

	// 4. Delete extraneous empty directories (in reverse depth order)
	for _, dirRel := range plan.DirsToDelete {
		select {
		case <-ctx.Done():
			res.Success = false
			return res, ctx.Err()
		default:
		}

		targetDir := filepath.Join(normReplica, filepath.FromSlash(dirRel))
		_ = filesystem.SafeRemoveDir(normReplica, targetDir)
	}

	res.Duration = time.Since(start)
	return res, nil
}
