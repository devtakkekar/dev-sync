package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicCopyFile_AndHash(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "subdir", "dest.txt")

	content := []byte("Hello Dev-Sync atomic file replacement!")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Copy
	err := AtomicCopyFile(src, dst, 64, true, true)
	if err != nil {
		t.Fatalf("AtomicCopyFile failed: %v", err)
	}

	// Verify content
	readBack, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile dst failed: %v", err)
	}
	if string(readBack) != string(content) {
		t.Errorf("expected %q, got %q", string(content), string(readBack))
	}

	// Compute hashes
	srcHash, err := ComputeFileHash(src, 64)
	if err != nil {
		t.Fatal(err)
	}
	dstHash, err := ComputeFileHash(dst, 64)
	if err != nil {
		t.Fatal(err)
	}

	if srcHash != dstHash {
		t.Errorf("hashes mismatch: %s vs %s", srcHash, dstHash)
	}
}

func TestSafeRemoveFile_SafetyCheck(t *testing.T) {
	tmpDir := t.TempDir()
	replica := filepath.Join(tmpDir, "replica")
	outside := filepath.Join(tmpDir, "outside.txt")

	if err := os.MkdirAll(replica, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("danger"), 0644); err != nil {
		t.Fatal(err)
	}

	// Attempting to remove file outside replica
	err := SafeRemoveFile(replica, outside)
	if err == nil {
		t.Errorf("expected safety error when deleting outside replica root")
	}

	// Valid remove inside replica
	inside := filepath.Join(replica, "file.txt")
	if err := os.WriteFile(inside, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := SafeRemoveFile(replica, inside); err != nil {
		t.Errorf("expected successful deletion inside replica, got: %v", err)
	}
}
