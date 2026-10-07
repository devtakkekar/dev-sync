package drift

import (
	"context"
	"fmt"

	"github.com/devsync/dev-sync/internal/executor"
	"github.com/devsync/dev-sync/internal/ignore"
	"github.com/devsync/dev-sync/internal/planner"
	"github.com/devsync/dev-sync/internal/scanner"
)

// DriftReport details differences caused by replica-side drift or missed syncs.
type DriftReport struct {
	ReplicaID   string   `json:"replicaId"`
	ReplicaPath string   `json:"replicaPath"`
	HasDrift    bool     `json:"hasDrift"`
	Modified    []string `json:"modified"`
	Missing     []string `json:"missing"`
	Extra       []string `json:"extra"`
}

// DetectDrift checks if a replica has diverged from source.
func DetectDrift(replicaID, sourceRoot, replicaRoot string, matcher *ignore.IgnoreMatcher, symlinkMode string) (*DriftReport, error) {
	srcScan, err := scanner.ScanDirectory(sourceRoot, matcher, symlinkMode, false)
	if err != nil {
		return nil, fmt.Errorf("failed to scan source: %w", err)
	}

	repScan, err := scanner.ScanDirectory(replicaRoot, matcher, symlinkMode, false)
	if err != nil {
		return nil, fmt.Errorf("failed to scan replica: %w", err)
	}

	report := &DriftReport{
		ReplicaID:   replicaID,
		ReplicaPath: replicaRoot,
		HasDrift:    false,
		Modified:    []string{},
		Missing:     []string{},
		Extra:       []string{},
	}

	for relPath, srcInfo := range srcScan.Files {
		repInfo, exists := repScan.Files[relPath]
		if !exists {
			report.Missing = append(report.Missing, relPath)
			report.HasDrift = true
			continue
		}

		if srcInfo.Size != repInfo.Size {
			report.Modified = append(report.Modified, relPath)
			report.HasDrift = true
		}
	}

	for relPath := range repScan.Files {
		if _, exists := srcScan.Files[relPath]; !exists {
			report.Extra = append(report.Extra, relPath)
			report.HasDrift = true
		}
	}

	return report, nil
}

// RepairDrift repairs the replica by strictly syncing from source.
func RepairDrift(ctx context.Context, sourceRoot, replicaID, replicaPath string, matcher *ignore.IgnoreMatcher, symlinkMode string, bufferSizeKB int) (*executor.ExecuteResult, error) {
	srcScan, err := scanner.ScanDirectory(sourceRoot, matcher, symlinkMode, false)
	if err != nil {
		return nil, err
	}

	repScan, err := scanner.ScanDirectory(replicaPath, matcher, symlinkMode, false)
	if err != nil {
		return nil, err
	}

	plan, err := planner.BuildPlan(replicaID, replicaPath, srcScan, repScan, planner.PlanOptions{
		DeleteExtraneous: true,
		DeepCompare:      true,
		BufferSizeKB:     bufferSizeKB,
	})
	if err != nil {
		return nil, err
	}

	return executor.Execute(ctx, sourceRoot, plan, executor.ExecuteOptions{
		DryRun:              false,
		BufferSizeKB:        bufferSizeKB,
		PreserveTimestamps:  true,
		PreservePermissions: true,
		VerifyAfterSync:     true,
	})
}
