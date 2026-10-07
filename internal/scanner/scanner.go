package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/devsync/dev-sync/internal/filesystem"
	"github.com/devsync/dev-sync/internal/ignore"
	"github.com/devsync/dev-sync/internal/safety"
)

// FileInfo holds metadata and optional hash for a file in the sync hierarchy.
type FileInfo struct {
	RelPath  string      // Normalized with forward slash (e.g. "src/main.go")
	FullPath string      // Absolute filesystem path
	Size     int64       // Size in bytes
	ModTime  time.Time   // Last modified time
	Mode     os.FileMode // File permissions/mode
	IsDir    bool
	Hash     string // SHA-256 (computed lazily or when requested)
}

// ScanResult holds the scanned files and directories.
type ScanResult struct {
	Files       map[string]*FileInfo // relPath -> FileInfo
	Directories []string             // sorted list of relative directory paths
	TotalBytes  int64
}

// ScanDirectory traverses the given directory tree, filtering items using matcher.
func ScanDirectory(root string, matcher *ignore.IgnoreMatcher, symlinkMode string, computeHashes bool) (*ScanResult, error) {
	normRoot, err := safety.NormalizePath(root)
	if err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	result := &ScanResult{
		Files:       make(map[string]*FileInfo),
		Directories: []string{},
	}

	// Verify root directory exists
	fi, err := os.Stat(normRoot)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", normRoot)
	}

	if matcher == nil {
		matcher = ignore.NewMatcher()
	}

	err = filepath.WalkDir(normRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Skip root itself
		if path == normRoot {
			return nil
		}

		rel, err := filepath.Rel(normRoot, path)
		if err != nil {
			return err
		}
		relNorm := filepath.ToSlash(rel)

		isDir := d.IsDir()

		// Check symlink
		if d.Type()&os.ModeSymlink != 0 {
			switch symlinkMode {
			case "preserve":
				// Reserved for future symlink preservation
			case "copy-target":
				// Follow symlink
				targetInfo, statErr := os.Stat(path)
				if statErr == nil {
					isDir = targetInfo.IsDir()
				}
			default: // "skip"
				if isDir {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Check ignore pattern
		if matcher.ShouldIgnore(relNorm, isDir) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		if isDir {
			result.Directories = append(result.Directories, relNorm)
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		finfo := &FileInfo{
			RelPath:  relNorm,
			FullPath: path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Mode:     info.Mode(),
			IsDir:    false,
		}

		if computeHashes {
			hash, err := filesystem.ComputeFileHash(path, 64)
			if err != nil {
				return fmt.Errorf("failed to compute hash for %s: %w", path, err)
			}
			finfo.Hash = hash
		}

		result.Files[relNorm] = finfo
		result.TotalBytes += finfo.Size

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
