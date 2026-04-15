package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// ErrHookTimeout is the sentinel error returned when a pre or verify hook
// exceeds the run's combined --timeout budget. Callers map this to the
// terminal StateTimeout and write timeout.flag.
var ErrHookTimeout = errors.New("hook timed out")

// HookPhase identifies which hook phase failed so callers can tag runner.log
// with the offending phase.
type HookPhase string

const (
	HookPhasePre    HookPhase = "pre-hook"
	HookPhaseVerify HookPhase = "verify-hook"
)

// HookTimeoutError carries the offending hook's phase, command index and
// command string. It unwraps to ErrHookTimeout so callers can switch on
// errors.Is(err, ErrHookTimeout).
type HookTimeoutError struct {
	Phase   HookPhase
	Index   int
	Command string
}

func (e *HookTimeoutError) Error() string {
	return fmt.Sprintf("%s %d (%s) timed out", e.Phase, e.Index, e.Command)
}

func (e *HookTimeoutError) Unwrap() error { return ErrHookTimeout }

// HookResult records the outcome of a hook command execution.
type HookResult struct {
	Command  string
	ExitCode int
	TimedOut bool
}

// RunPreHooks executes pre-commands in order, bounded by deadline. Halts on
// the first failure or the first timeout. A zero deadline disables enforcement.
func RunPreHooks(ctx context.Context, deadline time.Time, grace time.Duration, commands []string, workDir string, logWriter io.Writer) ([]HookResult, error) {
	results := make([]HookResult, 0, len(commands))
	for i, cmd := range commands {
		result := runHook(ctx, deadline, grace, cmd, workDir, logWriter)
		results = append(results, result)
		if result.TimedOut {
			return results, &HookTimeoutError{Phase: HookPhasePre, Index: i, Command: cmd}
		}
		if result.ExitCode != 0 {
			return results, fmt.Errorf("pre-command failed: %s (exit %d)", cmd, result.ExitCode)
		}
	}
	return results, nil
}

// RunVerifyHooks executes verify commands in order, bounded by deadline.
// Normal failures are collected and the rest still run, mirroring the prior
// behavior. A timeout is terminal: iteration halts because the run has
// already exceeded its budget.
func RunVerifyHooks(ctx context.Context, deadline time.Time, grace time.Duration, commands []string, workDir string, logWriter io.Writer) ([]HookResult, error) {
	results := make([]HookResult, 0, len(commands))
	var firstErr error
	for i, cmd := range commands {
		result := runHook(ctx, deadline, grace, cmd, workDir, logWriter)
		results = append(results, result)
		if result.TimedOut {
			return results, &HookTimeoutError{Phase: HookPhaseVerify, Index: i, Command: cmd}
		}
		if result.ExitCode != 0 && firstErr == nil {
			firstErr = fmt.Errorf("verify-command failed: %s (exit %d)", cmd, result.ExitCode)
		}
	}
	return results, firstErr
}

// runHook executes a single shell command bounded by deadline. On deadline
// it sends SIGTERM to the child process group, waits up to grace, then
// escalates with SIGKILL — matching the runDetachedProcess discipline.
func runHook(ctx context.Context, deadline time.Time, grace time.Duration, command, workDir string, logWriter io.Writer) HookResult {
	hookCtx := ctx
	if !deadline.IsZero() {
		var cancel context.CancelFunc
		hookCtx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	// Pre-Start guard: if the shared budget is already exhausted (an earlier
	// hook consumed it) or the caller canceled, refuse to launch the shell.
	// Otherwise a fast command could run side effects after the declared
	// --timeout window, and a select race on a past-deadline context could
	// still record the result as non-timeout. See BUG-060.
	if err := hookCtx.Err(); err != nil {
		return HookResult{
			Command:  command,
			ExitCode: -1,
			TimedOut: errors.Is(err, context.DeadlineExceeded),
		}
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = workDir
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	// Setpgid puts the shell and any descendants into a new process group so
	// signals delivered to -pid reach the whole tree.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return HookResult{Command: command, ExitCode: -1}
	}
	pgid := cmd.Process.Pid

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err := <-waitCh:
		// Re-check the deadline: if it fired while the command was running
		// and the waitCh branch happened to win the select race, still tag
		// the result as TimedOut so the caller maps to StateTimeout.
		timedOut := errors.Is(hookCtx.Err(), context.DeadlineExceeded)
		return resultFromWait(command, err, timedOut)

	case <-hookCtx.Done():
		isDeadline := errors.Is(hookCtx.Err(), context.DeadlineExceeded)
		_ = syscall.Kill(-pgid, syscall.SIGTERM)

		select {
		case err := <-waitCh:
			return resultFromWait(command, err, isDeadline)
		case <-time.After(grace):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			select {
			case err := <-waitCh:
				return resultFromWait(command, err, isDeadline)
			case <-time.After(5 * time.Second):
				return HookResult{Command: command, ExitCode: -1, TimedOut: isDeadline}
			}
		}
	}
}

func resultFromWait(command string, err error, timedOut bool) HookResult {
	if err == nil {
		return HookResult{Command: command, ExitCode: 0, TimedOut: timedOut}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return HookResult{Command: command, ExitCode: exitErr.ExitCode(), TimedOut: timedOut}
	}
	return HookResult{Command: command, ExitCode: -1, TimedOut: timedOut}
}
