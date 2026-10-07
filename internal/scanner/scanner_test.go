package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devsync/dev-sync/internal/ignore"
)

func TestScanDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file structure
	srcFile := filepath.Join(tmpDir, "hello.txt")
	subFile := filepath.Join(tmpDir, "sub", "world.txt")
	ignoredFile := filepath.Join(tmpDir, "test.log")
	devsyncFile := filepath.Join(tmpDir, ".devsync", "config.json")

	if err := os.MkdirAll(filepath.Dir(subFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(devsyncFile), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(srcFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subFile, []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignoredFile, []byte("log data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(devsyncFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	matcher := ignore.NewMatcher()
	matcher.AddRule("*.log")

	res, err := ScanDirectory(tmpDir, matcher, "skip", true)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Should contain hello.txt and sub/world.txt
	if len(res.Files) != 2 {
		t.Fatalf("expected 2 files, found %d: %+v", len(res.Files), res.Files)
	}

	if _, ok := res.Files["hello.txt"]; !ok {
		t.Errorf("missing hello.txt")
	}
	if _, ok := res.Files["sub/world.txt"]; !ok {
		t.Errorf("missing sub/world.txt")
	}

	// Should have computed hash
	if res.Files["hello.txt"].Hash == "" {
		t.Errorf("expected hash to be computed")
	}

	// Should ignore test.log and .devsync
	if _, ok := res.Files["test.log"]; ok {
		t.Errorf("test.log was not ignored")
	}
	if _, ok := res.Files[".devsync/config.json"]; ok {
		t.Errorf(".devsync/config.json was not ignored")
	}
}
