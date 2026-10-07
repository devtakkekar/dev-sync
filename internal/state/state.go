package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/devsync/dev-sync/internal/config"
)

const CurrentStateVersion = 1

// ReplicaState records the last known health and synchronization stats of a replica.
type ReplicaState struct {
	Status                 string    `json:"status"` // "in_sync", "out_of_sync", "unavailable", "error", "disabled"
	LastSuccessfulSync     time.Time `json:"lastSuccessfulSync,omitempty"`
	LastVerification       time.Time `json:"lastVerification,omitempty"`
	LastVerificationStatus string    `json:"lastVerificationStatus,omitempty"`
	FilesCount             int       `json:"filesCount"`
	BytesCount             int64     `json:"bytesCount"`
	LastError              string    `json:"lastError,omitempty"`
}

// State represents the project runtime state stored in .devsync/state.json.
type State struct {
	Version  int                      `json:"version"`
	LastSync time.Time                `json:"lastSync"`
	Replicas map[string]*ReplicaState `json:"replicas"`
}

// NewState creates an initialized empty State.
func NewState() *State {
	return &State{
		Version:  CurrentStateVersion,
		LastSync: time.Time{},
		Replicas: make(map[string]*ReplicaState),
	}
}

// LoadState reads .devsync/state.json. If missing, returns a fresh state without error.
func LoadState(projectRoot string) (*State, error) {
	statePath := filepath.Join(projectRoot, config.ConfigDirName, config.StateFileName)
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewState(), nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	st := NewState()
	if err := json.Unmarshal(data, st); err != nil {
		// Corrupted state - return fresh state and log warning
		return NewState(), fmt.Errorf("corrupted state file, will need reset: %w", err)
	}

	if st.Replicas == nil {
		st.Replicas = make(map[string]*ReplicaState)
	}

	return st, nil
}

// SaveState atomically writes state to .devsync/state.json.
func SaveState(projectRoot string, st *State) error {
	dir := filepath.Join(projectRoot, config.ConfigDirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode state: %w", err)
	}
	data = append(data, '\n')

	tmpPath := filepath.Join(dir, config.StateFileName+".tmp")
	destPath := filepath.Join(dir, config.StateFileName)

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to atomically update state file: %w", err)
	}

	return nil
}

// SetReplicaStatus updates the recorded status of a replica.
func (st *State) SetReplicaStatus(replicaID, status string, errStr string) {
	rep, ok := st.Replicas[replicaID]
	if !ok {
		rep = &ReplicaState{}
		st.Replicas[replicaID] = rep
	}
	rep.Status = status
	rep.LastError = errStr
}
