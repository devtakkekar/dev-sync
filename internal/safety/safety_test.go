package safety

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd error: %v", err)
	}

	norm, err := NormalizePath(".")
	if err != nil {
		t.Fatalf("NormalizePath error: %v", err)
	}
	if norm != filepath.Clean(wd) {
		t.Errorf("expected %s, got %s", wd, norm)
	}

	_, err = NormalizePath("   ")
	if err == nil {
		t.Errorf("expected error on empty path")
	}
}

func TestValidateReplicaPath(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	replicaDir := filepath.Join(tmpDir, "replica")
	nestedInSource := filepath.Join(sourceDir, "subreplica")
	parentOfSource := tmpDir

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(replicaDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Identical
	if err := ValidateReplicaPath(sourceDir, sourceDir); err == nil {
		t.Errorf("expected error when replica == source")
	}

	// Replica inside source
	if err := ValidateReplicaPath(sourceDir, nestedInSource); err == nil {
		t.Errorf("expected error when replica is inside source")
	}

	// Source inside replica
	if err := ValidateReplicaPath(sourceDir, parentOfSource); err == nil {
		t.Errorf("expected error when source is inside replica")
	}

	// Valid separate replica
	if err := ValidateReplicaPath(sourceDir, replicaDir); err != nil {
		t.Errorf("expected valid replica, got: %v", err)
	}
}

func TestSafeResolvePath(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Valid child
	resolved, err := SafeResolvePath(baseDir, "subdir/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(baseDir, "subdir", "file.txt")
	if resolved != expected {
		t.Errorf("expected %s, got %s", expected, resolved)
	}

	// Traversal attempt with ../../
	_, err = SafeResolvePath(baseDir, "../secret.txt")
	if err == nil {
		t.Errorf("expected error on path traversal")
	}

	// Traversal inside nested
	_, err = SafeResolvePath(baseDir, "subdir/../../secret.txt")
	if err == nil {
		t.Errorf("expected error on nested path traversal")
	}
}
