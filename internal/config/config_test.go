package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_DefaultAndSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	replicaDir := filepath.Join(tmpDir, "replica1")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig(sourceDir)
	cfg.Replicas = append(cfg.Replicas, ReplicaConfig{
		ID:      "test-rep",
		Path:    replicaDir,
		Enabled: true,
	})

	if err := cfg.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	if err := SaveConfig(sourceDir, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify discovery
	foundRoot, err := FindProjectRoot(sourceDir)
	if err != nil {
		t.Fatalf("FindProjectRoot failed: %v", err)
	}
	if foundRoot != sourceDir {
		t.Errorf("expected root %s, got %s", sourceDir, foundRoot)
	}

	// Load
	loaded, err := LoadConfig(sourceDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if loaded.Source != sourceDir {
		t.Errorf("expected source %s, got %s", sourceDir, loaded.Source)
	}
	if len(loaded.Replicas) != 1 || loaded.Replicas[0].ID != "test-rep" {
		t.Errorf("replica not loaded correctly: %+v", loaded.Replicas)
	}
}

func TestConfig_MaskWebhook(t *testing.T) {
	url := "https://discord.com/api/webhooks/123456789/mysecrettoken"
	masked := MaskWebhookURL(url)
	if masked == url {
		t.Errorf("url was not masked: %s", masked)
	}
	if filepath.Base(masked) != "********" {
		t.Errorf("expected masked tail, got: %s", masked)
	}
}
