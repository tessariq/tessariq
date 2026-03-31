package tmux

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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
// If command is non-empty, tmux starts the session by running that command.
func NewSession(ctx context.Context, name string, command []string) error {
	cmd := exec.CommandContext(ctx, "tmux", newSessionArgs(name, command)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create tmux session %q: %s: %w", name, out, err)
	}
	return nil
}

func newSessionArgs(name string, command []string) []string {
	args := []string{"new-session", "-d", "-s", name}
	if len(command) > 0 {
		args = append(args, shellCommand(command))
	}
	return args
}

func shellCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
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
