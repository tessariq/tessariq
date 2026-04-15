package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// StateInfo captures the minimum container state needed for orphan recovery.
type StateInfo struct {
	Exists     bool
	Running    bool
	ExitCode   int
	FinishedAt time.Time
}

type inspectState struct {
	Running    bool   `json:"Running"`
	ExitCode   int    `json:"ExitCode"`
	FinishedAt string `json:"FinishedAt"`
}

// InspectState returns runtime state for a named container. Missing containers
// are reported as Exists=false rather than an error.
func InspectState(ctx context.Context, name string) (StateInfo, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .State}}", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && isNotFoundError(trimmed) {
			return StateInfo{}, nil
		}
		return StateInfo{}, fmt.Errorf("inspect container %s: %s: %w", name, trimmed, err)
	}

	var raw inspectState
	if err := json.Unmarshal(out, &raw); err != nil {
		return StateInfo{}, fmt.Errorf("parse container state for %s: %w", name, err)
	}

	var finishedAt time.Time
	if raw.FinishedAt != "" && raw.FinishedAt != "0001-01-01T00:00:00Z" {
		parsed, err := time.Parse(time.RFC3339Nano, raw.FinishedAt)
		if err != nil {
			return StateInfo{}, fmt.Errorf("parse container finished_at for %s: %w", name, err)
		}
		finishedAt = parsed
	}

	return StateInfo{
		Exists:     true,
		Running:    raw.Running,
		ExitCode:   raw.ExitCode,
		FinishedAt: finishedAt,
	}, nil
}

// Remove deletes a named container if it exists.
func Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && isNotFoundError(trimmed) {
			return nil
		}
		return fmt.Errorf("remove container %s: %s: %w", name, trimmed, err)
	}
	return nil
}

// isNotFoundError detects docker's "container/object not found" error across
// docker-cli variants. Docker Desktop on macOS/Linux emits
// "Error: No such object: <name>" / "Error response from daemon: No such
// container: <name>", while Alpine's docker-cli package emits lowercase
// "error: no such object: <name>". Match case-insensitively so both are
// treated as "container does not exist" rather than a hard error.
func isNotFoundError(trimmed string) bool {
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "no such object") || strings.Contains(lower, "no such container")
}
