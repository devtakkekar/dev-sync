package planner

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/devsync/dev-sync/internal/filesystem"
	"github.com/devsync/dev-sync/internal/scanner"
)

// ActionType defines the sync action to perform on a path.
type ActionType string

const (
	ActionCreate    ActionType = "CREATE"
	ActionUpdate    ActionType = "UPDATE"
	ActionDelete    ActionType = "DELETE"
	ActionUnchanged ActionType = "UNCHANGED"
)

// PlanItem represents an individual file action in the synchronization plan.
type PlanItem struct {
	RelPath      string            `json:"relPath"`
	Action       ActionType        `json:"action"`
	SourceInfo   *scanner.FileInfo `json:"sourceInfo,omitempty"`
	ReplicaInfo  *scanner.FileInfo `json:"replicaInfo,omitempty"`
	Reason       string            `json:"reason"`
	SizeDiff     int64             `json:"sizeDiff"`
}

// SyncPlan represents the complete plan for a replica before execution.
type SyncPlan struct {
	ReplicaID     string                `json:"replicaId"`
	ReplicaPath   string                `json:"replicaPath"`
	Items         []*PlanItem           `json:"items"`
	DirsToCreate  []string              `json:"dirsToCreate"`
	DirsToDelete  []string              `json:"dirsToDelete"`
	Counts        map[ActionType]int    `json:"counts"`
	TotalBytesSync int64                 `json:"totalBytesSync"`
}

// PlanOptions configures plan generation.
type PlanOptions struct {
	DeleteExtraneous bool
	DeepCompare      bool // Force SHA-256 hash check even if sizes & modtimes match
	BufferSizeKB     int
}

// BuildPlan compares source scan result against replica scan result and builds a validated SyncPlan.
func BuildPlan(replicaID, replicaPath string, sourceScan, replicaScan *scanner.ScanResult, opts PlanOptions) (*SyncPlan, error) {
	plan := &SyncPlan{
		ReplicaID:   replicaID,
		ReplicaPath: replicaPath,
		Items:       []*PlanItem{},
		Counts: map[ActionType]int{
			ActionCreate:    0,
			ActionUpdate:    0,
			ActionDelete:    0,
			ActionUnchanged: 0,
		},
	}

	// 1. Process files present in source
	for relPath, srcInfo := range sourceScan.Files {
		repInfo, existsInReplica := replicaScan.Files[relPath]

		if !existsInReplica {
			// File is missing in replica -> CREATE
			item := &PlanItem{
				RelPath:    relPath,
				Action:     ActionCreate,
				SourceInfo: srcInfo,
				Reason:     "File missing in replica",
				SizeDiff:   srcInfo.Size,
			}
			plan.Items = append(plan.Items, item)
			plan.Counts[ActionCreate]++
			plan.TotalBytesSync += srcInfo.Size
			continue
		}

		// File exists in both source and replica
		if srcInfo.Size != repInfo.Size {
			// Size mismatch -> UPDATE
			item := &PlanItem{
				RelPath:     relPath,
				Action:      ActionUpdate,
				SourceInfo:  srcInfo,
				ReplicaInfo: repInfo,
				Reason:      fmt.Sprintf("Size differs (src: %d, rep: %d)", srcInfo.Size, repInfo.Size),
				SizeDiff:    srcInfo.Size - repInfo.Size,
			}
			plan.Items = append(plan.Items, item)
			plan.Counts[ActionUpdate]++
			plan.TotalBytesSync += srcInfo.Size
			continue
		}

		// Sizes match. Check modification time or hash.
		timeDiff := math.Abs(float64(srcInfo.ModTime.Sub(repInfo.ModTime) / time.Second))
		needsHashCheck := opts.DeepCompare || timeDiff > 1.0

		if needsHashCheck {
			srcHash := srcInfo.Hash
			var err error
			if srcHash == "" {
				srcHash, err = filesystem.ComputeFileHash(srcInfo.FullPath, opts.BufferSizeKB)
				if err != nil {
					return nil, fmt.Errorf("failed to hash source file %s: %w", srcInfo.FullPath, err)
				}
				srcInfo.Hash = srcHash
			}

			repHash := repInfo.Hash
			if repHash == "" {
				repHash, err = filesystem.ComputeFileHash(repInfo.FullPath, opts.BufferSizeKB)
				if err != nil {
					return nil, fmt.Errorf("failed to hash replica file %s: %w", repInfo.FullPath, err)
				}
				repInfo.Hash = repHash
			}

			if srcHash != repHash {
				item := &PlanItem{
					RelPath:     relPath,
					Action:      ActionUpdate,
					SourceInfo:  srcInfo,
					ReplicaInfo: repInfo,
					Reason:      "Content hash differs",
					SizeDiff:    0,
				}
				plan.Items = append(plan.Items, item)
				plan.Counts[ActionUpdate]++
				plan.TotalBytesSync += srcInfo.Size
				continue
			}
		}

		// Unchanged
		item := &PlanItem{
			RelPath:     relPath,
			Action:      ActionUnchanged,
			SourceInfo:  srcInfo,
			ReplicaInfo: repInfo,
			Reason:      "File content matches source",
			SizeDiff:    0,
		}
		plan.Items = append(plan.Items, item)
		plan.Counts[ActionUnchanged]++
	}

	// 2. Process extraneous files present in replica but missing in source
	if opts.DeleteExtraneous {
		for relPath, repInfo := range replicaScan.Files {
			if _, existsInSource := sourceScan.Files[relPath]; !existsInSource {
				item := &PlanItem{
					RelPath:     relPath,
					Action:      ActionDelete,
					ReplicaInfo: repInfo,
					Reason:      "File does not exist in source",
					SizeDiff:    -repInfo.Size,
				}
				plan.Items = append(plan.Items, item)
				plan.Counts[ActionDelete]++
			}
		}

		// Check directories in replica that don't exist in source
		srcDirMap := make(map[string]bool)
		for _, d := range sourceScan.Directories {
			srcDirMap[d] = true
		}
		// Also source files parent directories
		for r := range sourceScan.Files {
			parts := strings.Split(r, "/")
			for i := 1; i < len(parts); i++ {
				srcDirMap[strings.Join(parts[:i], "/")] = true
			}
		}

		for _, repDir := range replicaScan.Directories {
			if !srcDirMap[repDir] {
				plan.DirsToDelete = append(plan.DirsToDelete, repDir)
			}
		}
		// Sort dirs to delete in reverse order (deepest directories first)
		sort.Slice(plan.DirsToDelete, func(i, j int) bool {
			return len(plan.DirsToDelete[i]) > len(plan.DirsToDelete[j])
		})
	}

	// Directories to create
	repDirMap := make(map[string]bool)
	for _, d := range replicaScan.Directories {
		repDirMap[d] = true
	}
	for _, srcDir := range sourceScan.Directories {
		if !repDirMap[srcDir] {
			plan.DirsToCreate = append(plan.DirsToCreate, srcDir)
		}
	}
	// Sort dirs to create in normal order (parents first)
	sort.Strings(plan.DirsToCreate)

	// Sort items consistently by relative path
	sort.Slice(plan.Items, func(i, j int) bool {
		return plan.Items[i].RelPath < plan.Items[j].RelPath
	})

	return plan, nil
}
