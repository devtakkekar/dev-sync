package verify

import (
	"fmt"

	"github.com/devsync/dev-sync/internal/ignore"
	"github.com/devsync/dev-sync/internal/scanner"
)

// HashDifference records a mismatch between source and replica for a specific file.
type HashDifference struct {
	RelPath     string `json:"relPath"`
	SourceHash  string `json:"sourceHash"`
	ReplicaHash string `json:"replicaHash"`
}

// VerifyResult contains detailed comparison data between source and a replica.
type VerifyResult struct {
	ReplicaID     string           `json:"replicaId"`
	ReplicaPath   string           `json:"replicaPath"`
	InSync        bool             `json:"inSync"`
	TotalSource   int              `json:"totalSource"`
	TotalReplica  int              `json:"totalReplica"`
	MatchedFiles  int              `json:"matchedFiles"`
	ModifiedFiles []HashDifference `json:"modifiedFiles"`
	MissingFiles  []string         `json:"missingFiles"` // Present in source, missing in replica
	ExtraFiles    []string         `json:"extraFiles"`   // Present in replica, missing in source
}

// VerifyReplica computes deep cryptographic SHA-256 hashes of all non-ignored files
// and returns exact diffs.
func VerifyReplica(replicaID, sourceRoot, replicaRoot string, matcher *ignore.IgnoreMatcher, symlinkMode string) (*VerifyResult, error) {
	srcScan, err := scanner.ScanDirectory(sourceRoot, matcher, symlinkMode, true)
	if err != nil {
		return nil, fmt.Errorf("failed to scan source: %w", err)
	}

	repScan, err := scanner.ScanDirectory(replicaRoot, matcher, symlinkMode, true)
	if err != nil {
		return nil, fmt.Errorf("failed to scan replica: %w", err)
	}

	res := &VerifyResult{
		ReplicaID:     replicaID,
		ReplicaPath:   replicaRoot,
		InSync:        true,
		TotalSource:   len(srcScan.Files),
		TotalReplica:  len(repScan.Files),
		ModifiedFiles: []HashDifference{},
		MissingFiles:  []string{},
		ExtraFiles:    []string{},
	}

	for relPath, srcInfo := range srcScan.Files {
		repInfo, exists := repScan.Files[relPath]
		if !exists {
			res.MissingFiles = append(res.MissingFiles, relPath)
			res.InSync = false
			continue
		}

		if srcInfo.Hash != repInfo.Hash {
			res.ModifiedFiles = append(res.ModifiedFiles, HashDifference{
				RelPath:     relPath,
				SourceHash:  srcInfo.Hash,
				ReplicaHash: repInfo.Hash,
			})
			res.InSync = false
		} else {
			res.MatchedFiles++
		}
	}

	for relPath := range repScan.Files {
		if _, exists := srcScan.Files[relPath]; !exists {
			res.ExtraFiles = append(res.ExtraFiles, relPath)
			res.InSync = false
		}
	}

	return res, nil
}
