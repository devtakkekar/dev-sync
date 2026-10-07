package safety

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathEscape is returned when a target path resolves outside its base directory.
var ErrPathEscape = errors.New("target path escapes base directory")

// ErrInvalidReplica is returned when replica path is dangerous or overlaps with source.
var ErrInvalidReplica = errors.New("invalid replica path")

// NormalizePath cleans and returns a standardized absolute path.
func NormalizePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path cannot be empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	return filepath.Clean(abs), nil
}

// IsSubPath checks if child is strictly inside parent directory.
func IsSubPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	// If rel is "." or starts with "..", it's not a strict sub-path
	if rel == "." || strings.HasPrefix(rel, "..") || rel == "" {
		return false
	}
	return true
}

// ArePathsEqual checks if two paths resolve to the same normalized location.
func ArePathsEqual(pathA, pathB string) bool {
	normA, errA := NormalizePath(pathA)
	normB, errB := NormalizePath(pathB)
	if errA != nil || errB != nil {
		return filepath.Clean(pathA) == filepath.Clean(pathB)
	}
	return strings.EqualFold(normA, normB)
}

// ValidateReplicaPath ensures the replica directory is safe, does not collide with source,
// and is not a system root or dangerous system path.
func ValidateReplicaPath(sourceRoot, replicaPath string) error {
	normSource, err := NormalizePath(sourceRoot)
	if err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}

	normReplica, err := NormalizePath(replicaPath)
	if err != nil {
		return fmt.Errorf("invalid replica path: %w", err)
	}

	// 1. Replica cannot be identical to source
	if ArePathsEqual(normSource, normReplica) {
		return fmt.Errorf("%w: replica path cannot be identical to source path (%s)", ErrInvalidReplica, normReplica)
	}

	// 2. Prevent nested source inside replica or replica inside source
	if IsSubPath(normSource, normReplica) {
		return fmt.Errorf("%w: replica (%s) cannot be inside source directory (%s)", ErrInvalidReplica, normReplica, normSource)
	}
	if IsSubPath(normReplica, normSource) {
		return fmt.Errorf("%w: source (%s) cannot be inside replica directory (%s)", ErrInvalidReplica, normSource, normReplica)
	}

	// 3. Prevent root directory replication (danger of wiping root)
	volume := filepath.VolumeName(normReplica)
	rootOfReplica := filepath.Clean(volume + string(filepath.Separator))
	if normReplica == rootOfReplica || normReplica == "/" || normReplica == "\\" {
		return fmt.Errorf("%w: cannot use filesystem root as replica destination (%s)", ErrInvalidReplica, normReplica)
	}

	// 4. On Windows, check for protected system dirs like C:\Windows, C:\Program Files
	lower := strings.ToLower(normReplica)
	dangerousWindowsDirs := []string{
		`c:\windows`,
		`c:\winnt`,
		`c:\system volume information`,
		`c:\$recycle.bin`,
	}
	for _, dangerous := range dangerousWindowsDirs {
		if strings.HasPrefix(lower, dangerous) {
			return fmt.Errorf("%w: destination path inside protected system directory is forbidden (%s)", ErrInvalidReplica, normReplica)
		}
	}

	// 5. On Unix/Linux, check protected system dirs
	dangerousUnixDirs := []string{
		"/bin", "/sbin", "/boot", "/dev", "/etc", "/lib", "/lib64", "/proc", "/sys", "/root",
	}
	for _, dangerous := range dangerousUnixDirs {
		if lower == dangerous || strings.HasPrefix(lower, dangerous+"/") {
			return fmt.Errorf("%w: destination path inside protected system directory is forbidden (%s)", ErrInvalidReplica, normReplica)
		}
	}

	return nil
}

// SafeResolvePath verifies that childRelPath stays strictly inside baseDir.
// It returns the normalized absolute target path.
func SafeResolvePath(baseDir, childRelPath string) (string, error) {
	normBase, err := NormalizePath(baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %w", err)
	}

	// Clean the relative path to remove redundant slashes and . / ..
	cleanRel := filepath.Clean(filepath.FromSlash(childRelPath))
	if cleanRel == "." || cleanRel == "" {
		return normBase, nil
	}

	if filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("%w: absolute relative path not allowed: %s", ErrPathEscape, childRelPath)
	}

	combined := filepath.Join(normBase, cleanRel)
	normTarget, err := NormalizePath(combined)
	if err != nil {
		return "", fmt.Errorf("failed to normalize target path: %w", err)
	}

	rel, err := filepath.Rel(normBase, normTarget)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("%w: path %s resolves to %s outside %s", ErrPathEscape, childRelPath, normTarget, normBase)
	}

	return normTarget, nil
}

// IsSymlink checks if the given path is a symbolic link or Windows reparse point.
func IsSymlink(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return (fi.Mode() & os.ModeSymlink) != 0, nil
}
