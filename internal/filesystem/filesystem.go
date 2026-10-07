package filesystem

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/devsync/dev-sync/internal/safety"
)

// ErrTargetOutsideReplica is returned if an operation attempts to touch a file outside replica root.
var ErrTargetOutsideReplica = errors.New("target path is outside the replica root")

// EnsureDir creates the directory and any parent directories if they don't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// ComputeFileHash calculates the SHA-256 checksum of a file using streaming I/O.
func ComputeFileHash(path string, bufferSizeKB int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if bufferSizeKB <= 0 {
		bufferSizeKB = 64
	}
	buf := make([]byte, bufferSizeKB*1024)

	hasher := sha256.New()
	if _, err := io.CopyBuffer(hasher, f, buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// randomSuffix generates a short random string for temporary files.
func randomSuffix() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// AtomicCopyFile safely copies src to dst using a temporary file in the destination directory,
// flushing to storage, and performing an atomic rename.
func AtomicCopyFile(src, dst string, bufferSizeKB int, preserveTime, preservePerms bool) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// Create temp file in the same directory as the destination
	tmpName := fmt.Sprintf(".devsync-tmp.%s", randomSuffix())
	tmpPath := filepath.Join(dstDir, tmpName)

	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temporary file %s: %w", tmpPath, err)
	}

	// Ensure cleanup if anything fails before rename
	success := false
	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if bufferSizeKB <= 0 {
		bufferSizeKB = 64
	}
	buf := make([]byte, bufferSizeKB*1024)

	if _, err := io.CopyBuffer(tmpFile, srcFile, buf); err != nil {
		return fmt.Errorf("failed to copy data from %s to %s: %w", src, tmpPath, err)
	}

	// Flush buffers to physical storage
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file to disk: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if preservePerms {
		_ = os.Chmod(tmpPath, srcInfo.Mode())
	}

	if preserveTime {
		_ = os.Chtimes(tmpPath, srcInfo.ModTime(), srcInfo.ModTime())
	}

	// Perform atomic rename with retry on Windows in case of temporary file locks (e.g. antivirus/indexer)
	renameErr := atomicRenameWithRetry(tmpPath, dst, 3)
	if renameErr != nil {
		return fmt.Errorf("failed to atomically rename temp file to %s: %w", dst, renameErr)
	}

	success = true
	return nil
}

func atomicRenameWithRetry(src, dst string, maxRetries int) error {
	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err = os.Rename(src, dst)
		if err == nil {
			return nil
		}
		if attempt < maxRetries {
			time.Sleep(time.Duration((attempt+1)*25) * time.Millisecond)
		}
	}
	return err
}

// SafeRemoveFile removes targetPath only if it is strictly inside replicaRoot.
func SafeRemoveFile(replicaRoot, targetPath string) error {
	normReplica, err := safety.NormalizePath(replicaRoot)
	if err != nil {
		return fmt.Errorf("invalid replica root: %w", err)
	}

	normTarget, err := safety.NormalizePath(targetPath)
	if err != nil {
		return fmt.Errorf("invalid target path: %w", err)
	}

	if !safety.IsSubPath(normReplica, normTarget) {
		return fmt.Errorf("%w: cannot delete %s (not inside %s)", ErrTargetOutsideReplica, normTarget, normReplica)
	}

	if err := os.Remove(normTarget); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SafeRemoveDir removes empty targetDir only if it is strictly inside replicaRoot and targetDir != replicaRoot.
func SafeRemoveDir(replicaRoot, targetDir string) error {
	normReplica, err := safety.NormalizePath(replicaRoot)
	if err != nil {
		return fmt.Errorf("invalid replica root: %w", err)
	}

	normTarget, err := safety.NormalizePath(targetDir)
	if err != nil {
		return fmt.Errorf("invalid target directory: %w", err)
	}

	if normTarget == normReplica {
		return fmt.Errorf("%w: cannot remove replica root directory", ErrTargetOutsideReplica)
	}

	if !safety.IsSubPath(normReplica, normTarget) {
		return fmt.Errorf("%w: directory %s is not inside %s", ErrTargetOutsideReplica, normTarget, normReplica)
	}

	if err := os.Remove(normTarget); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SafeRemoveDirAll removes directory tree targetDir only if strictly inside replicaRoot and targetDir != replicaRoot.
func SafeRemoveDirAll(replicaRoot, targetDir string) error {
	normReplica, err := safety.NormalizePath(replicaRoot)
	if err != nil {
		return fmt.Errorf("invalid replica root: %w", err)
	}

	normTarget, err := safety.NormalizePath(targetDir)
	if err != nil {
		return fmt.Errorf("invalid target directory: %w", err)
	}

	if normTarget == normReplica {
		return fmt.Errorf("%w: cannot recursively remove replica root directory", ErrTargetOutsideReplica)
	}

	if !safety.IsSubPath(normReplica, normTarget) {
		return fmt.Errorf("%w: directory %s is not inside %s", ErrTargetOutsideReplica, normTarget, normReplica)
	}

	return os.RemoveAll(normTarget)
}
