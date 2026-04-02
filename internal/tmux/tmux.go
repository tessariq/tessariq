package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
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
// The call retries up to 3 times with short backoff to handle transient
// server restarts (e.g. server exiting after last session was killed).
func NewSession(ctx context.Context, name string, command []string) error {
	const maxAttempts = 3
	args := newSessionArgs(name, command)
	var lastOut string
	var lastErr error
	for attempt := range maxAttempts {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 100 * time.Millisecond):
			case <-ctx.Done():
				return fmt.Errorf("create tmux session %q: %w (last: %s: %w)", name, ctx.Err(), lastOut, lastErr)
			}
		}
		cmd := exec.CommandContext(ctx, "tmux", args...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		lastOut = strings.TrimSpace(string(out))
		lastErr = err
		if isDuplicateSessionError(lastOut) {
			break
		}
	}
	return fmt.Errorf("create tmux session %q: %s: %w", name, lastOut, lastErr)
}

// isDuplicateSessionError returns true when tmux reports the session name
// already exists. These errors must not be retried.
func isDuplicateSessionError(output string) bool {
	return strings.Contains(output, "duplicate session")
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

// AttachSession attaches the current terminal to an existing tmux session.
func AttachSession(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "tmux", "attach-session", "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("attach tmux session %q: %w", name, err)
	}
	return nil
}
