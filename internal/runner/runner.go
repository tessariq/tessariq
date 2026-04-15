package runner

import (
	"context"
	"errors"
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

type outputFileConfigurer interface {
	SetOutput(stdout, stderr *os.File)
}

type outputWriterConfigurer interface {
	SetOutputWriter(stdout, stderr io.Writer)
}

type logStreamStopper interface {
	StopLogStream() error
}

type processCleaner interface {
	Cleanup(ctx context.Context) error
}

type waitResult struct {
	exitCode int
	err      error
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
	ContainerName string          // recorded in evidence; used by CLI for direct docker attach
	SessionReady  chan<- struct{} // closed when ready for attach; nil = ignored
	Clock         func() time.Time
	LogCapBytes   int64 // 0 uses DefaultLogCapBytes
	// DiffArtifactWriter, when set, is invoked on every terminal state
	// before the terminal status is written. Failed, timed-out, and killed
	// runs must still emit diff evidence when changes exist because
	// promote.Run accepts any terminal state and requires diff artifacts.
	// A write failure escalates an otherwise-successful run to StateFailed
	// but never masks a pre-existing non-success state or its exit code.
	DiffArtifactWriter func(ctx context.Context, evidenceDir string) error
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

	// Create tmux session for non-interactive mode (log tailing).
	// For interactive mode, the session is created inside runInteractiveProcess
	// after the container starts, so docker attach can connect.
	if r.Session != nil && r.SessionName != "" && !r.Config.Interactive {
		fmt.Fprintf(logs.RunnerLog, "[%s] creating tmux session %s\n", r.clock().UTC().Format(time.RFC3339), r.SessionName)
		if err := r.Session.StartSession(ctx, r.SessionName, r.sessionCommand(logs.RunLogPath())); err != nil {
			finishedAt := r.clock()
			fmt.Fprintf(logs.RunnerLog, "[%s] tmux session creation failed: %s\n", finishedAt.UTC().Format(time.RFC3339), err)
			return r.writeTerminalStatus(StateFailed, startedAt, finishedAt, 1, false)
		}
		if r.SessionReady != nil {
			close(r.SessionReady)
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

		// Cap run.log post-hoc — the detached path bypasses CappedWriter
		// for direct fd streaming, so enforce the size limit here.
		if _, err := CapLogFile(logs.RunLogPath(), logs.CapBytes()); err != nil {
			fmt.Fprintf(logs.RunnerLog, "[%s] warning: cap run.log: %s\n",
				r.clock().UTC().Format(time.RFC3339), err)
		}
	}

	// Run verify hooks (only if process succeeded and not timed out).
	if processState == StateSuccess && len(r.Config.Verify) > 0 {
		fmt.Fprintf(logs.RunnerLog, "[%s] running %d verify-hook(s)\n", r.clock().UTC().Format(time.RFC3339), len(r.Config.Verify))
		if _, verifyErr := RunVerifyHooks(ctx, r.Config.Verify, r.RepoRoot, logs.RunnerLog); verifyErr != nil {
			fmt.Fprintf(logs.RunnerLog, "[%s] verify-hook failed: %s\n", r.clock().UTC().Format(time.RFC3339), verifyErr)
			processState = StateFailed
			exitCode = 1
			timedOut = false
		}
	}

	// Commit diff artifacts on every terminal state. promote.Run accepts
	// any terminal state and requires diff evidence when changes exist, so
	// a failed/timeout/killed run with file changes must still emit
	// artifacts. A diff-write failure only escalates state when the run
	// would otherwise be success — it must never mask a pre-existing
	// non-success state or its exit code.
	if r.DiffArtifactWriter != nil {
		if err := r.DiffArtifactWriter(ctx, r.EvidenceDir); err != nil {
			fmt.Fprintf(logs.RunnerLog, "[%s] diff-artifact write failed: %s\n", r.clock().UTC().Format(time.RFC3339), err)
			if processState == StateSuccess {
				processState = StateFailed
				exitCode = 1
				timedOut = false
			}
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

	directOutput := false
	// Prefer direct fd pass-through so docker logs writes to run.log
	// without a Go pipe intermediary — tail -f sees writes immediately.
	if proc, ok := r.Process.(outputFileConfigurer); ok {
		directOutput = true
		proc.SetOutput(logs.RunLogFile(), logs.RunLogFile())
	} else if proc, ok := r.Process.(outputWriterConfigurer); ok {
		proc.SetOutputWriter(logs.RunLog, logs.RunLog)
	}

	fmt.Fprintf(logs.RunLogFile(), "[%s] process output start\n", r.clock().UTC().Format(time.RFC3339))
	defer func() {
		fmt.Fprintf(logs.RunLogFile(), "[%s] process output end\n", r.clock().UTC().Format(time.RFC3339))
	}()

	if err := r.Process.Start(timeoutCtx); err != nil {
		fmt.Fprintf(logs.RunnerLog, "[%s] process start failed: %s\n", r.clock().UTC().Format(time.RFC3339), err)
		return 1, false, StateFailed
	}

	// Log "starting process" only after Start returns successfully so
	// lifecycle.processStartObserved distinguishes "container is about to
	// be created" (no log line yet, Live=true) from "container existed and
	// was pruned" (log line present, fail closed).
	fmt.Fprintf(logs.RunnerLog, "[%s] starting process (timeout=%s, grace=%s)\n",
		r.clock().UTC().Format(time.RFC3339), r.Config.Timeout, r.Config.Grace)

	stopLogCapMonitor := func() {}
	if directOutput {
		stopLogCapMonitor = r.startDetachedLogCapMonitor(logs, r.Process)
	}
	defer stopLogCapMonitor()

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
		if errors.Is(timeoutCtx.Err(), context.Canceled) && ctx.Err() != nil {
			return r.handleContextCancellation(ctx, logs, waitCh)
		}

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

func (r *Runner) handleContextCancellation(ctx context.Context, logs *LogFiles, waitCh <-chan waitResult) (exitCode int, timedOut bool, state State) {
	state = SignalStateFromCause(context.Cause(ctx))
	fmt.Fprintf(logs.RunnerLog, "[%s] context cancelled, sending SIGTERM\n", r.clock().UTC().Format(time.RFC3339))
	_ = r.Process.Signal(syscall.SIGTERM)

	select {
	case result := <-waitCh:
		return result.exitCode, false, state
	case <-time.After(r.Config.Grace):
		fmt.Fprintf(logs.RunnerLog, "[%s] grace period expired, sending SIGKILL\n", r.clock().UTC().Format(time.RFC3339))
		_ = r.Process.Signal(os.Kill)
		select {
		case result := <-waitCh:
			return result.exitCode, false, state
		case <-time.After(5 * time.Second):
			return -1, false, state
		}
	}
}

func (r *Runner) startDetachedLogCapMonitor(logs *LogFiles, process ProcessRunner) func() {
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()

		stopper, canStopStream := process.(logStreamStopper)
		warnedStopFailure := false

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				info, err := os.Stat(logs.RunLogPath())
				if err != nil || info.Size() <= logs.CapBytes() {
					continue
				}

				truncated, err := CapLogFile(logs.RunLogPath(), logs.CapBytes())
				if err != nil {
					fmt.Fprintf(logs.RunnerLog, "[%s] warning: cap detached run.log: %s\n",
						r.clock().UTC().Format(time.RFC3339), err)
					continue
				}
				if truncated {
					fmt.Fprintf(logs.RunnerLog, "[%s] detached run.log reached cap; truncating active stream\n",
						r.clock().UTC().Format(time.RFC3339))
				}

				if !canStopStream {
					continue
				}
				if err := stopper.StopLogStream(); err != nil {
					if !warnedStopFailure {
						fmt.Fprintf(logs.RunnerLog, "[%s] warning: stop detached log stream: %s\n",
							r.clock().UTC().Format(time.RFC3339), err)
						warnedStopFailure = true
					}
					continue
				}
				return
			}
		}
	}()

	return func() {
		close(stopCh)
		<-doneCh
	}
}

func (r *Runner) runInteractiveProcess(ctx context.Context, startedAt time.Time, logs *LogFiles) (exitCode int, timedOut bool, state State) {
	timer := NewActivityTimer(r.Config.Timeout)

	// Set up output with activity tracking via the capped writer.
	aw := NewActivityWriter(logs.RunLog, timer)
	if proc, ok := r.Process.(outputWriterConfigurer); ok {
		proc.SetOutputWriter(aw, aw)
	}

	fmt.Fprintf(logs.RunLog, "[%s] process output start\n", r.clock().UTC().Format(time.RFC3339))
	defer func() {
		fmt.Fprintf(logs.RunLog, "[%s] process output end\n", r.clock().UTC().Format(time.RFC3339))
	}()

	// Start activity timer before the process so RecordActivity calls from
	// docker log streaming find a properly initialized timer.
	timer.Start()
	defer timer.Stop()

	if err := r.Process.Start(ctx); err != nil {
		fmt.Fprintf(logs.RunnerLog, "[%s] process start failed: %s\n", r.clock().UTC().Format(time.RFC3339), err)
		return 1, false, StateFailed
	}

	// Log "starting interactive process" only after Start returns
	// successfully so lifecycle.processStartObserved distinguishes
	// "container is about to be created" (no log line yet, Live=true)
	// from "container existed and was pruned" (log line present, fail
	// closed).
	fmt.Fprintf(logs.RunnerLog, "[%s] starting interactive process (activity-timeout=%s, grace=%s)\n",
		r.clock().UTC().Format(time.RFC3339), r.Config.Timeout, r.Config.Grace)

	// Wait for process in a goroutine — started before session creation
	// so we can drain it if session creation fails.
	waitCh := make(chan waitResult, 1)
	go func() {
		code, err := r.Process.Wait()
		waitCh <- waitResult{code, err}
	}()

	// Create tmux session BEFORE signaling ready, so the attach function
	// can find it immediately. For interactive mode the session command is
	// docker attach (user interacts with the agent TUI); for non-interactive
	// it tails the run log.
	if r.Session != nil && r.SessionName != "" {
		fmt.Fprintf(logs.RunnerLog, "[%s] creating tmux session %s\n", r.clock().UTC().Format(time.RFC3339), r.SessionName)
		if err := r.Session.StartSession(ctx, r.SessionName, r.sessionCommand(logs.RunLogPath())); err != nil {
			fmt.Fprintf(logs.RunnerLog, "[%s] tmux session creation failed (non-fatal): %s\n", r.clock().UTC().Format(time.RFC3339), err)
		}
	}

	// Signal ready for attach — tmux session exists.
	if r.SessionReady != nil {
		close(r.SessionReady)
	}

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
		return r.handleContextCancellation(ctx, logs, waitCh)
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
	if cleaner, ok := r.Process.(processCleaner); ok {
		if err := cleaner.Cleanup(context.Background()); err != nil {
			return fmt.Errorf("cleanup process: %w", err)
		}
	}
	if state != StateSuccess {
		return &TerminalStateError{State: state, ExitCode: exitCode}
	}
	return nil
}
