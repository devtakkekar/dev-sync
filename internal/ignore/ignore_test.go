package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreMatcher_Defaults(t *testing.T) {
	m := NewMatcher()

	// .devsync should always be ignored
	if !m.ShouldIgnore(".devsync", true) {
		t.Errorf("expected .devsync dir to be ignored")
	}
	if !m.ShouldIgnore(".devsync/config.json", false) {
		t.Errorf("expected .devsync/config.json to be ignored")
	}
	if !m.ShouldIgnore(".git/HEAD", false) {
		t.Errorf("expected .git/HEAD to be ignored")
	}
	if !m.ShouldIgnore(".devsync-tmp.12345", false) {
		t.Errorf("expected temp file to be ignored")
	}

	// Normal source file shouldn't be ignored
	if m.ShouldIgnore("src/main.go", false) {
		t.Errorf("src/main.go should not be ignored")
	}
}

func TestIgnoreMatcher_CustomRules(t *testing.T) {
	m := NewMatcher()
	m.AddRule("node_modules/")
	m.AddRule("*.log")
	m.AddRule("build/**")
	m.AddRule("!important.log")

	if !m.ShouldIgnore("node_modules/pkg/index.js", false) {
		t.Errorf("expected node_modules/pkg/index.js to be ignored")
	}
	if !m.ShouldIgnore("app.log", false) {
		t.Errorf("expected app.log to be ignored")
	}
	if m.ShouldIgnore("important.log", false) {
		t.Errorf("expected important.log to NOT be ignored (negated)")
	}
	if !m.ShouldIgnore("build/output.bin", false) {
		t.Errorf("expected build/output.bin to be ignored")
	}
	if m.ShouldIgnore("src/app.js", false) {
		t.Errorf("expected src/app.js to NOT be ignored")
	}
}

func TestLoadIgnoreFile(t *testing.T) {
	tmpDir := t.TempDir()
	ignorePath := filepath.Join(tmpDir, ".devsyncignore")
	content := "# comment line\n\nvendor/\n*.bak\n"
	if err := os.WriteFile(ignorePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadIgnoreFile(ignorePath)
	if err != nil {
		t.Fatalf("failed to load ignore file: %v", err)
	}

	if !m.ShouldIgnore("vendor/lib.go", false) {
		t.Errorf("expected vendor/lib.go to be ignored")
	}
	if !m.ShouldIgnore("test.bak", false) {
		t.Errorf("expected test.bak to be ignored")
	}
	if m.ShouldIgnore("test.go", false) {
		t.Errorf("expected test.go to NOT be ignored")
	}
}
