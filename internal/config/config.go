package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devsync/dev-sync/internal/safety"
)

const (
	CurrentConfigVersion = 1
	ConfigDirName        = ".devsync"
	ConfigFileName       = "config.json"
	StateFileName        = "state.json"
	LogDirName           = "logs"
	IgnoreFileName       = ".devsyncignore"
)

// ReplicaConfig defines an individual target replica directory.
type ReplicaConfig struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

// SyncConfig defines file copy and removal behavior.
type SyncConfig struct {
	DeleteExtraneous    bool `json:"deleteExtraneous"`
	VerifyAfterSync     bool `json:"verifyAfterSync"`
	AtomicWrites        bool `json:"atomicWrites"`
	PreserveTimestamps  bool `json:"preserveTimestamps"`
	PreservePermissions bool `json:"preservePermissions"`
}

// DriftConfig defines out-of-sync drift detection and recovery options.
type DriftConfig struct {
	AutoRepair    bool `json:"autoRepair"`
	DetectOnWatch bool `json:"detectOnWatch"`
}

// WatchConfig defines live monitoring options.
type WatchConfig struct {
	DebounceMs int  `json:"debounceMs"`
	Source     bool `json:"source"`
	Replicas   bool `json:"replicas"`
}

// VerificationConfig defines background/periodic integrity verification.
type VerificationConfig struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"intervalSeconds"`
}

// FilesystemConfig defines filesystem handling options.
type FilesystemConfig struct {
	SymlinkMode  string `json:"symlinkMode"`  // "skip", "preserve", "copy-target"
	BufferSizeKB int    `json:"bufferSizeKB"` // e.g. 64 or 128
}

// DiscordEventsConfig defines which events trigger notifications.
type DiscordEventsConfig struct {
	SyncStarted        bool `json:"syncStarted"`
	SyncCompleted      bool `json:"syncCompleted"`
	SyncFailed         bool `json:"syncFailed"`
	DriftDetected      bool `json:"driftDetected"`
	DriftRepaired      bool `json:"driftRepaired"`
	ReplicaUnavailable bool `json:"replicaUnavailable"`
}

// DiscordConfig holds webhook configuration.
type DiscordConfig struct {
	Enabled    bool                `json:"enabled"`
	WebhookURL string              `json:"webhookUrl"`
	Events     DiscordEventsConfig `json:"events"`
}

// Config represents the top-level project configuration.
type Config struct {
	Version      int                `json:"version"`
	Source       string             `json:"source"`
	Replicas     []ReplicaConfig    `json:"replicas"`
	Sync         SyncConfig         `json:"sync"`
	Drift        DriftConfig        `json:"drift"`
	Watch        WatchConfig        `json:"watch"`
	Verification VerificationConfig `json:"verification"`
	Filesystem   FilesystemConfig   `json:"filesystem"`
	Discord      DiscordConfig      `json:"discord"`
}

// DefaultConfig returns a new Config with safe production defaults.
func DefaultConfig(sourcePath string) *Config {
	return &Config{
		Version: CurrentConfigVersion,
		Source:  sourcePath,
		Replicas: []ReplicaConfig{},
		Sync: SyncConfig{
			DeleteExtraneous:    true,
			VerifyAfterSync:     true,
			AtomicWrites:        true,
			PreserveTimestamps:  true,
			PreservePermissions: true,
		},
		Drift: DriftConfig{
			AutoRepair:    false,
			DetectOnWatch: true,
		},
		Watch: WatchConfig{
			DebounceMs: 150,
			Source:     true,
			Replicas:   false,
		},
		Verification: VerificationConfig{
			Enabled:         false,
			IntervalSeconds: 300,
		},
		Filesystem: FilesystemConfig{
			SymlinkMode:  "skip",
			BufferSizeKB: 64,
		},
		Discord: DiscordConfig{
			Enabled:    false,
			WebhookURL: "",
			Events: DiscordEventsConfig{
				SyncStarted:        false,
				SyncCompleted:      true,
				SyncFailed:         true,
				DriftDetected:      true,
				DriftRepaired:      true,
				ReplicaUnavailable: true,
			},
		},
	}
}

// EffectiveWebhookURL returns the webhook URL, checking the DEVSYNC_DISCORD_WEBHOOK_URL env var first.
func (c *Config) EffectiveWebhookURL() string {
	if env := strings.TrimSpace(os.Getenv("DEVSYNC_DISCORD_WEBHOOK_URL")); env != "" {
		return env
	}
	return strings.TrimSpace(c.Discord.WebhookURL)
}

// MaskWebhookURL masks sensitive webhook secrets when printing or logging.
func MaskWebhookURL(url string) string {
	url = strings.TrimSpace(url)
	if url == "" {
		return "(none)"
	}
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		parts[len(parts)-1] = "********"
		return strings.Join(parts, "/")
	}
	return "********"
}

// FindProjectRoot searches upwards from currentDir for a directory containing .devsync/config.json.
func FindProjectRoot(startDir string) (string, error) {
	curr, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(curr, ConfigDirName, ConfigFileName)
		if fi, err := os.Stat(configPath); err == nil && !fi.IsDir() {
			return curr, nil
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			// Reached filesystem root
			break
		}
		curr = parent
	}

	return "", errors.New("no Dev-Sync project found (run 'dev-sync init' first)")
}

// LoadConfig reads and parses .devsync/config.json from projectRoot.
func LoadConfig(projectRoot string) (*Config, error) {
	configPath := filepath.Join(projectRoot, ConfigDirName, ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration JSON: %w", err)
	}

	// Apply defaults for fields that might be missing in older configs
	if cfg.Watch.DebounceMs <= 0 {
		cfg.Watch.DebounceMs = 150
	}
	if cfg.Filesystem.BufferSizeKB <= 0 {
		cfg.Filesystem.BufferSizeKB = 64
	}
	if cfg.Filesystem.SymlinkMode == "" {
		cfg.Filesystem.SymlinkMode = "skip"
	}

	return cfg, nil
}

// SaveConfig atomically writes cfg to .devsync/config.json in projectRoot.
func SaveConfig(projectRoot string, cfg *Config) error {
	dir := filepath.Join(projectRoot, ConfigDirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	data = append(data, '\n')

	tmpPath := filepath.Join(dir, ConfigFileName+".tmp")
	destPath := filepath.Join(dir, ConfigFileName)

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		// Cleanup tmp on error
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to atomically update config file: %w", err)
	}

	return nil
}

// Validate validates the configuration for correctness and safety.
func (c *Config) Validate() error {
	if c.Source == "" {
		return errors.New("source directory cannot be empty")
	}

	normSource, err := safety.NormalizePath(c.Source)
	if err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}

	seenIDs := make(map[string]bool)
	seenPaths := make(map[string]bool)

	for i, r := range c.Replicas {
		id := strings.TrimSpace(r.ID)
		if id == "" {
			return fmt.Errorf("replica at index %d has empty ID", i)
		}
		if seenIDs[id] {
			return fmt.Errorf("duplicate replica ID: %s", id)
		}
		seenIDs[id] = true

		if r.Path == "" {
			return fmt.Errorf("replica '%s' has empty path", id)
		}

		normReplica, err := safety.NormalizePath(r.Path)
		if err != nil {
			return fmt.Errorf("replica '%s' has invalid path: %w", id, err)
		}

		if seenPaths[normReplica] {
			return fmt.Errorf("duplicate replica path: %s", normReplica)
		}
		seenPaths[normReplica] = true

		// Check safety
		if err := safety.ValidateReplicaPath(normSource, normReplica); err != nil {
			return fmt.Errorf("replica '%s': %w", id, err)
		}
	}

	switch c.Filesystem.SymlinkMode {
	case "skip", "preserve", "copy-target":
	default:
		return fmt.Errorf("invalid symlinkMode '%s' (allowed: skip, preserve, copy-target)", c.Filesystem.SymlinkMode)
	}

	return nil
}

// FindReplicaByID looks up a replica configuration by its ID.
func (c *Config) FindReplicaByID(id string) (*ReplicaConfig, int) {
	for i, r := range c.Replicas {
		if strings.EqualFold(r.ID, id) {
			return &c.Replicas[i], i
		}
	}
	return nil, -1
}
