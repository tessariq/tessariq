package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/tessariq/tessariq/internal/run"
)

// ProcessRunner is an abstraction for starting and managing an external process.
type ProcessRunner interface {
	Start(ctx context.Context) error
	Wait() (int, error)
	Signal(sig os.Signal) error
}

// SessionStarter creates a tmux session for the run.
type SessionStarter interface {
	StartSession(ctx context.Context, sessionName string, command []string) error
}

type outputWriterConfigurer interface {
	SetOutputWriter(stdout, stderr io.Writer)
}

// Runner orchestrates the full run lifecycle.
type Runner struct {
	RunID         string
	EvidenceDir   string
	RepoRoot      string // repository root for hook CWD
	Config        run.Config
	Process       ProcessRunner
	Session       SessionStarter
	SessionName   string
	ContainerName string // needed for interactive tmux session command
	Clock         func() time.Time
	LogCapBytes   int64 // 0 uses DefaultLogCapBytes
}

func (r *Runner) clock() time.Time {
	if r.Clock != nil {
		return r.Clock()
	}
	return time.Now()
}

// Run executes the runner lifecycle:
// 1. Write initial status.json
// 2. Open durable logs
// 3. Run pre-hooks
// 4. Start and wait for process (if provided)
// 5. Monitor timeout
// 6. Run verify hooks
// 7. Write final status.json
func (r *Runner) Run(ctx context.Context) error {
	startedAt := r.clock()

	// Write initial status so it exists even on subsequent failure.
	if err := WriteStatus(r.EvidenceDir, NewInitialStatus(startedAt)); err != nil {
		return fmt.Errorf("write initial status: %w", err)
	}

	// Open durable log files with write-time capping.
	logs, err := OpenLogs(r.EvidenceDir, r.LogCapBytes)
	if err != nil {
		return fmt.Errorf("open logs: %w", err)
	}
	defer logs.Close()

	fmt.Fprintf(logs.RunnerLog, "[%s] runner started for run %s\n", startedAt.UTC().Format(time.RFC3339), r.RunID)

	// Create tmux session if a session starter is configured.
	if r.Session != nil && r.SessionName != "" {
		fmt.Fprintf(logs.RunnerLog, "[%s] creating tmux session %s\n", r.clock().UTC().Format(time.RFC3339), r.SessionName)
		if err := r.Session.StartSession(ctx, r.SessionName, r.sessionCommand(logs.RunLogPath())); err != nil {
			finishedAt := r.clock()
			fmt.Fprintf(logs.RunnerLog, "[%s] tmux session creation failed: %s\n", finishedAt.UTC().Format(time.RFC3339), err)
			return r.writeTerminalStatus(StateFailed, startedAt, finishedAt, 1, false)
		}
	}

	// Run pre-hooks.
	if len(r.Config.Pre) > 0 {
		fmt.Fprintf(logs.RunnerLog, "[%s] running %d pre-hook(s)\n", r.clock().UTC().Format(time.RFC3339), len(r.Config.Pre))
		_, preErr := RunPreHooks(ctx, r.Config.Pre, r.RepoRoot, logs.RunnerLog)
		if preErr != nil {
			finishedAt := r.clock()
			fmt.Fprintf(logs.RunnerLog, "[%s] pre-hook failed: %s\n", finishedAt.UTC().Format(time.RFC3339), preErr)
			return r.writeTerminalStatus(StateFailed, startedAt, finishedAt, 1, false)
		}
	}

	// Run process if provided.
	exitCode := 0
	timedOut := false
	processState := StateSuccess

	if r.Process != nil {
		exitCode, timedOut, processState = r.runProcess(ctx, startedAt, logs)
	}

	// Run verify hooks (only if process succeeded and not timed out).
	if processState == StateSuccess && len(r.Config.Verify) > 0 {
		fmt.Fprintf(logs.RunnerLog, "[%s] running %d verify-hook(s)\n", r.clock().UTC().Format(time.RFC3339), len(r.Config.Verify))
		_, verifyErr := RunVerifyHooks(ctx, r.Config.Verify, r.RepoRoot, logs.RunnerLog)
		if verifyErr != nil {
			finishedAt := r.clock()
			fmt.Fprintf(logs.RunnerLog, "[%s] verify-hook failed: %s\n", finishedAt.UTC().Format(time.RFC3339), verifyErr)
			return r.writeTerminalStatus(StateFailed, startedAt, finishedAt, 1, false)
		}
	}

	finishedAt := r.clock()
	fmt.Fprintf(logs.RunnerLog, "[%s] runner finished with state %s\n", finishedAt.UTC().Format(time.RFC3339), processState)
	return r.writeTerminalStatus(processState, startedAt, finishedAt, exitCode, timedOut)
}

func (r *Runner) runProcess(ctx context.Context, startedAt time.Time, logs *LogFiles) (exitCode int, timedOut bool, state State) {
	if r.Config.Interactive {
		return r.runInteractiveProcess(ctx, startedAt, logs)
	}
	return r.runDetachedProcess(ctx, startedAt, logs)
}

func (r *Runner) runDetachedProcess(ctx context.Context, startedAt time.Time, logs *LogFiles) (exitCode int, timedOut bool, state State) {
	// Create timeout context.
	timeoutCtx, cancel := context.WithTimeout(ctx, r.Config.Timeout)
	defer cancel()

	fmt.Fprintf(logs.RunnerLog, "[%s] starting process (timeout=%s, grace=%s)\n",
		r.clock().UTC().Format(time.RFC3339), r.Config.Timeout, r.Config.Grace)

	if proc, ok := r.Process.(outputWriterConfigurer); ok {
		proc.SetOutputWriter(logs.RunLog, logs.RunLog)
	}

	fmt.Fprintf(logs.RunLog, "[%s] process output start\n", r.clock().UTC().Format(time.RFC3339))
	defer func() {
		fmt.Fprintf(logs.RunLog, "[%s] process output end\n", r.clock().UTC().Format(time.RFC3339))
	}()

	if err := r.Process.Start(timeoutCtx); err != nil {
		fmt.Fprintf(logs.RunnerLog, "[%s] process start failed: %s\n", r.clock().UTC().Format(time.RFC3339), err)
		return 1, false, StateFailed
	}

	// Wait for process in a goroutine.
	type waitResult struct {
		exitCode int
		err      error
	}
	waitCh := make(chan waitResult, 1)
	go func() {
		code, err := r.Process.Wait()
		waitCh <- waitResult{code, err}
	}()

	select {
	case result := <-waitCh:
		// Process finished before timeout.
		if result.exitCode != 0 {
			return result.exitCode, false, StateFailed
		}
		return 0, false, StateSuccess

	case <-timeoutCtx.Done():
		// Timeout expired -- escalate with SIGTERM then SIGKILL.
		fmt.Fprintf(logs.RunnerLog, "[%s] timeout reached, writing timeout flag\n", r.clock().UTC().Format(time.RFC3339))
		_ = WriteTimeoutFlag(r.EvidenceDir)

		// Step 1: graceful SIGTERM.
		fmt.Fprintf(logs.RunnerLog, "[%s] sending SIGTERM\n", r.clock().UTC().Format(time.RFC3339))
		_ = r.Process.Signal(syscall.SIGTERM)

		select {
		case result := <-waitCh:
			// Exited after SIGTERM — no SIGKILL needed.
			return result.exitCode, true, StateTimeout
		case <-time.After(r.Config.Grace):
			// Grace expired — escalate to SIGKILL.
			fmt.Fprintf(logs.RunnerLog, "[%s] grace period expired, sending SIGKILL\n", r.clock().UTC().Format(time.RFC3339))
			_ = r.Process.Signal(os.Kill)
			select {
			case result := <-waitCh:
				return result.exitCode, true, StateTimeout
			case <-time.After(5 * time.Second):
				return -1, true, StateTimeout
			}
		}
	}
}

func (r *Runner) runInteractiveProcess(ctx context.Context, startedAt time.Time, logs *LogFiles) (exitCode int, timedOut bool, state State) {
	timer := NewActivityTimer(r.Config.Timeout)

	fmt.Fprintf(logs.RunnerLog, "[%s] starting interactive process (activity-timeout=%s, grace=%s)\n",
		r.clock().UTC().Format(time.RFC3339), r.Config.Timeout, r.Config.Grace)

	// Set up output with activity tracking via the capped writer.
	aw := NewActivityWriter(logs.RunLog, timer)
	if proc, ok := r.Process.(outputWriterConfigurer); ok {
		proc.SetOutputWriter(aw, aw)
	}

	fmt.Fprintf(logs.RunLog, "[%s] process output start\n", r.clock().UTC().Format(time.RFC3339))
	defer func() {
		fmt.Fprintf(logs.RunLog, "[%s] process output end\n", r.clock().UTC().Format(time.RFC3339))
	}()

	if err := r.Process.Start(ctx); err != nil {
		fmt.Fprintf(logs.RunnerLog, "[%s] process start failed: %s\n", r.clock().UTC().Format(time.RFC3339), err)
		return 1, false, StateFailed
	}

	timer.Start()
	defer timer.Stop()

	// Wait for process in a goroutine.
	type waitResult struct {
		exitCode int
		err      error
	}
	waitCh := make(chan waitResult, 1)
	go func() {
		code, err := r.Process.Wait()
		waitCh <- waitResult{code, err}
	}()

	select {
	case result := <-waitCh:
		// Process finished before timeout.
		if result.exitCode != 0 {
			return result.exitCode, false, StateFailed
		}
		return 0, false, StateSuccess

	case <-timer.Expired():
		// Active time exceeded -- escalate with SIGTERM then SIGKILL.
		fmt.Fprintf(logs.RunnerLog, "[%s] activity timeout reached (active time: %s), writing timeout flag\n",
			r.clock().UTC().Format(time.RFC3339), timer.Elapsed())
		_ = WriteTimeoutFlag(r.EvidenceDir)

		// Step 1: graceful SIGTERM.
		fmt.Fprintf(logs.RunnerLog, "[%s] sending SIGTERM\n", r.clock().UTC().Format(time.RFC3339))
		_ = r.Process.Signal(syscall.SIGTERM)

		select {
		case result := <-waitCh:
			return result.exitCode, true, StateTimeout
		case <-time.After(r.Config.Grace):
			// Grace expired — escalate to SIGKILL.
			fmt.Fprintf(logs.RunnerLog, "[%s] grace period expired, sending SIGKILL\n", r.clock().UTC().Format(time.RFC3339))
			_ = r.Process.Signal(os.Kill)
			select {
			case result := <-waitCh:
				return result.exitCode, true, StateTimeout
			case <-time.After(5 * time.Second):
				return -1, true, StateTimeout
			}
		}

	case <-ctx.Done():
		// Parent context cancelled — still use graceful escalation.
		fmt.Fprintf(logs.RunnerLog, "[%s] context cancelled, sending SIGTERM\n", r.clock().UTC().Format(time.RFC3339))
		_ = r.Process.Signal(syscall.SIGTERM)
		select {
		case result := <-waitCh:
			return result.exitCode, false, StateKilled
		case <-time.After(r.Config.Grace):
			fmt.Fprintf(logs.RunnerLog, "[%s] grace period expired, sending SIGKILL\n", r.clock().UTC().Format(time.RFC3339))
			_ = r.Process.Signal(os.Kill)
			select {
			case result := <-waitCh:
				return result.exitCode, false, StateKilled
			case <-time.After(5 * time.Second):
				return -1, false, StateKilled
			}
		}
	}
}

func (r *Runner) sessionCommand(runLogPath string) []string {
	if r.Config.Interactive && r.ContainerName != "" {
		return []string{"docker", "attach", r.ContainerName}
	}
	return []string{"tail", "-n", "+1", "-f", runLogPath}
}

func (r *Runner) writeTerminalStatus(state State, startedAt, finishedAt time.Time, exitCode int, timedOut bool) error {
	s := NewTerminalStatus(state, startedAt, finishedAt, exitCode, timedOut)
	if err := WriteStatus(r.EvidenceDir, s); err != nil {
		return fmt.Errorf("write terminal status: %w", err)
	}
	return nil
}
