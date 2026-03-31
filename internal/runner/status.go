package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State represents a run lifecycle state.
type State string

const (
	StateRunning     State = "running"
	StateSuccess     State = "success"
	StateFailed      State = "failed"
	StateTimeout     State = "timeout"
	StateKilled      State = "killed"
	StateInterrupted State = "interrupted"
)

// IsTerminal returns true if the state is one of the five terminal states.
func (s State) IsTerminal() bool {
	switch s {
	case StateSuccess, StateFailed, StateTimeout, StateKilled, StateInterrupted:
		return true
	default:
		return false
	}
}

// Status represents the status.json evidence artifact.
type Status struct {
	SchemaVersion int    `json:"schema_version"`
	State         State  `json:"state"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	ExitCode      int    `json:"exit_code"`
	TimedOut      bool   `json:"timed_out"`
}

// NewInitialStatus creates a non-terminal status recording the start time.
func NewInitialStatus(startedAt time.Time) Status {
	return Status{
		SchemaVersion: 1,
		State:         StateRunning,
		StartedAt:     startedAt.UTC().Format(time.RFC3339),
	}
}

// NewTerminalStatus creates a completed status with all required fields.
func NewTerminalStatus(state State, startedAt, finishedAt time.Time, exitCode int, timedOut bool) Status {
	return Status{
		SchemaVersion: 1,
		State:         state,
		StartedAt:     startedAt.UTC().Format(time.RFC3339),
		FinishedAt:    finishedAt.UTC().Format(time.RFC3339),
		ExitCode:      exitCode,
		TimedOut:      timedOut,
	}
}

// WriteStatus writes status.json to the evidence directory atomically.
func WriteStatus(evidenceDir string, s Status) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}

	target := filepath.Join(evidenceDir, "status.json")
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write status temp file: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("rename status file: %w", err)
	}

	return nil
}

// ReadStatus reads and parses status.json from the evidence directory.
func ReadStatus(evidenceDir string) (Status, error) {
	data, err := os.ReadFile(filepath.Join(evidenceDir, "status.json"))
	if err != nil {
		return Status{}, fmt.Errorf("read status: %w", err)
	}

	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return Status{}, fmt.Errorf("parse status: %w", err)
	}

	return s, nil
}
