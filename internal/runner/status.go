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
	// CleanupError, when non-empty, records that container cleanup
	// (docker rm -f) failed after the run's primary work finished. An
	// otherwise-successful run is downgraded to StateFailed so the CLI
	// exit code, status.json, and promote eligibility agree; the field
	// carries the original cleanup error for operator diagnostics and is
	// also set on non-success runs so a post-mortem sees the cleanup
	// fault without masking the primary terminal state.
	CleanupError string `json:"cleanup_error,omitempty"`
}

// Validate checks that the status has a supported schema version and
// all spec-required fields are present.
func (s Status) Validate() error {
	if s.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", s.SchemaVersion)
	}
	if s.State == "" {
		return fmt.Errorf("missing required field %q", "state")
	}
	if s.StartedAt == "" {
		return fmt.Errorf("missing required field %q", "started_at")
	}
	return nil
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

// TerminalStateError is returned by Runner.Run when the run completes with
// a non-success terminal state. It is not an infrastructure error — status.json
// has been written successfully. Cause, when non-nil, carries the original
// underlying error (for example a container cleanup failure that forced an
// otherwise-successful run to be downgraded to StateFailed).
type TerminalStateError struct {
	State    State
	ExitCode int
	Cause    error
}

func (e *TerminalStateError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("run finished with state %s (exit code %d): %s", e.State, e.ExitCode, e.Cause)
	}
	return fmt.Sprintf("run finished with state %s (exit code %d)", e.State, e.ExitCode)
}

// Unwrap exposes the underlying cause so callers can use errors.Is/errors.As.
func (e *TerminalStateError) Unwrap() error {
	return e.Cause
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
