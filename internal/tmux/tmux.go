package tmux

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ErrTmuxNotAvailable indicates that the tmux binary is not on PATH.
var ErrTmuxNotAvailable = errors.New("tmux is not installed or not in PATH; install tmux to use tessariq run")

// Available checks whether the tmux binary is on PATH.
func Available() error {
	_, err := exec.LookPath("tmux")
	if err != nil {
		return ErrTmuxNotAvailable
	}
	return nil
}

// NewSession creates a new detached tmux session with the given name.
func NewSession(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create tmux session %q: %s: %w", name, out, err)
	}
	return nil
}

// HasSession returns true if a tmux session with the given name exists.
func HasSession(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", name)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, fmt.Errorf("check tmux session %q: %w", name, err)
}

// KillSession destroys a tmux session. It is best-effort and does not
// fail if the session does not exist.
func KillSession(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "tmux", "kill-session", "-t", name)
	_ = cmd.Run()
	return nil
}
