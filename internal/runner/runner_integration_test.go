//go:build integration

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/testutil"
	"github.com/tessariq/tessariq/internal/tmux"
)

func TestMain(m *testing.M) {
	cleanup := testutil.SetupIsolatedTmuxServer()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// shellProcess runs a shell command as the ProcessRunner for integration tests.
type shellProcess struct {
	command      string
	cmd          *exec.Cmd
	stdoutWriter io.Writer
	stderrWriter io.Writer
}

func newShellProcess(command string) *shellProcess {
	return &shellProcess{command: command}
}

func (s *shellProcess) Start(_ context.Context) error {
	// Use exec.Command (not CommandContext) so the process lifecycle is
	// controlled exclusively by Signal, matching production container behavior
	// where docker stop/kill are the only shutdown mechanism.
	s.cmd = exec.Command("sh", "-c", s.command)
	s.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	s.cmd.Stdout = s.stdoutWriter
	s.cmd.Stderr = s.stderrWriter
	return s.cmd.Start()
}

func (s *shellProcess) Wait() (int, error) {
	err := s.cmd.Wait()
	if err != nil {
		// ProcessState is set even when Wait returns a pipe-copy error
		// alongside a successful process exit.
		if s.cmd.ProcessState != nil {
			return s.cmd.ProcessState.ExitCode(), nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

func (s *shellProcess) Signal(sig os.Signal) error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	// Send signal to the process group so child processes also receive it,
	// mirroring docker stop/kill behavior on container process trees.
	return syscall.Kill(-s.cmd.Process.Pid, sig.(syscall.Signal))
}

func (s *shellProcess) SetOutputWriter(stdout, stderr io.Writer) {
	s.stdoutWriter = stdout
	s.stderrWriter = stderr
}

func newIntegrationRunner(dir string, proc ProcessRunner) *Runner {
	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	return &Runner{
		RunID:       "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		EvidenceDir: dir,
		RepoRoot:    dir,
		Config:      cfg,
		Process:     proc,
	}
}

func TestRunnerIntegration_SuccessProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 0"))

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State)
	require.Equal(t, 0, s.ExitCode)
	require.False(t, s.TimedOut)
}

func TestRunnerIntegration_FailedProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 7"))

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)
	require.Equal(t, 7, termErr.ExitCode)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
	require.Equal(t, 7, s.ExitCode)
}

func TestRunnerIntegration_TimeoutWritesFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("sleep 60"))
	r.Config.Timeout = 100 * time.Millisecond
	r.Config.Grace = 50 * time.Millisecond

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateTimeout, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	_, err = os.Stat(filepath.Join(dir, "timeout.flag"))
	require.NoError(t, err, "timeout.flag must exist")
}

func TestRunnerIntegration_TimeoutSIGTERMExitsGracefully(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// sleep exits on SIGTERM — process should stop without SIGKILL.
	r := newIntegrationRunner(dir, newShellProcess("sleep 60"))
	r.Config.Timeout = 100 * time.Millisecond
	r.Config.Grace = 2 * time.Second

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateTimeout, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	// Verify runner log shows SIGTERM was sent.
	logData, err := os.ReadFile(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
	require.Contains(t, string(logData), "sending SIGTERM")
}

func TestRunnerIntegration_TimeoutEscalationToSIGKILL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// trap '' TERM makes the process ignore SIGTERM, forcing SIGKILL escalation.
	r := newIntegrationRunner(dir, newShellProcess("trap '' TERM; sleep 60"))
	r.Config.Timeout = 100 * time.Millisecond
	r.Config.Grace = 200 * time.Millisecond

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateTimeout, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	// Verify runner log shows both SIGTERM and SIGKILL.
	logData, err := os.ReadFile(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
	require.Contains(t, string(logData), "sending SIGTERM")
	require.Contains(t, string(logData), "sending SIGKILL")
}

func TestRunnerIntegration_ContextCancellationWritesInterruptedStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("trap 'exit 130' TERM; sleep 60"))
	r.Config.Timeout = time.Minute
	r.Config.Grace = 250 * time.Millisecond

	ctx, cancel := context.WithCancelCause(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel(SignalCause(syscall.SIGINT))
	}()

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(ctx), &termErr)
	require.Equal(t, StateInterrupted, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateInterrupted, s.State)
	require.False(t, s.TimedOut)

	logData, err := os.ReadFile(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
	require.Contains(t, string(logData), "context cancelled")
	require.NotContains(t, string(logData), "timeout reached")
}

func TestRunnerIntegration_EvidenceDurability(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 1"))

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)

	// status.json must exist
	_, err := os.Stat(filepath.Join(dir, "status.json"))
	require.NoError(t, err)

	// run.log must exist
	_, err = os.Stat(filepath.Join(dir, "run.log"))
	require.NoError(t, err)

	// runner.log must exist
	_, err = os.Stat(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)

	// status.json must be valid JSON with required fields
	data, err := os.ReadFile(filepath.Join(dir, "status.json"))
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	requiredFields := []string{"schema_version", "state", "started_at", "finished_at", "exit_code", "timed_out"}
	for _, field := range requiredFields {
		require.Contains(t, raw, field, "status.json must contain %s", field)
	}
}

func TestRunnerIntegration_EvidenceCompletenessAllRequired(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write the artifacts that the runner does NOT produce (they come from
	// other parts of the pipeline: manifest, agent, runtime, workspace, task).
	extraFiles := map[string]string{
		"manifest.json":  `{"schema_version":1}`,
		"agent.json":     `{"schema_version":1}`,
		"runtime.json":   `{"schema_version":1}`,
		"workspace.json": `{"schema_version":1}`,
		"task.md":        "# Task\nDo something.",
	}
	for name, content := range extraFiles {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	r := newIntegrationRunner(dir, newShellProcess("printf test-output"))
	require.NoError(t, r.Run(context.Background()))

	// All 8 required files must be present and non-empty.
	err := CheckEvidenceCompleteness(dir)
	require.NoError(t, err)

	// Validate JSON schema_version on all JSON artifacts.
	jsonFiles := []string{"manifest.json", "status.json", "agent.json", "runtime.json", "workspace.json"}
	for _, name := range jsonFiles {
		data, readErr := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, readErr, "%s read", name)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &raw), "%s JSON parse", name)
		require.Contains(t, raw, "schema_version", "%s must have schema_version", name)
	}
}

func TestRunnerIntegration_PreHookWithRealProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 0"))
	r.Config.Pre = []string{"echo pre-hook-ran"}

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State)

	// runner.log should contain pre-hook output
	logData, err := os.ReadFile(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
	require.Contains(t, string(logData), "pre-hook-ran")
}

func TestRunnerIntegration_PreHookFailurePreventsProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("echo should-not-run"))
	r.Config.Pre = []string{"exit 1"}

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
}

func TestRunnerIntegration_ProcessOutputWrittenToRunLog(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("printf process-output"))

	require.NoError(t, r.Run(context.Background()))

	data, err := os.ReadFile(filepath.Join(dir, "run.log"))
	require.NoError(t, err)
	require.Contains(t, string(data), "process-output")
}

func TestRunnerIntegration_VerifyHookFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 0"))
	r.Config.Verify = []string{"exit 1"}

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
}

func TestRunnerIntegration_TmuxSessionCreated(t *testing.T) {
	// Host-tool guard is intentional here: these tests exercise Runner's tmux
	// session management against the real host binary, not a container. Full
	// CLI e2e tests use containers.StartRunEnv which provides tmux inside the
	// container. See AGENTS.md for the distinction.
	testutil.RequireTmux(t)

	ctx := context.Background()
	sessionName := "tessariq-test-runner-tmux-" + t.Name()
	t.Cleanup(func() { _ = tmux.KillSession(ctx, sessionName) })

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 0"))
	r.Session = &tmux.Starter{}
	r.SessionName = sessionName

	require.NoError(t, r.Run(ctx))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State)

	exists, err := tmux.HasSession(ctx, sessionName)
	require.NoError(t, err)
	require.True(t, exists, "tmux session should exist after runner completes")
}

func TestRunnerIntegration_TmuxSessionExistsAfterProcessFails(t *testing.T) {
	testutil.RequireTmux(t)

	ctx := context.Background()
	sessionName := "tessariq-test-runner-fail-" + t.Name()
	t.Cleanup(func() { _ = tmux.KillSession(ctx, sessionName) })

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 1"))
	r.Session = &tmux.Starter{}
	r.SessionName = sessionName

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(ctx), &termErr)
	require.Equal(t, StateFailed, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)

	exists, err := tmux.HasSession(ctx, sessionName)
	require.NoError(t, err)
	require.True(t, exists, "tmux session should persist even when process fails")
}

func TestRunnerIntegration_TmuxSessionShowsRunLogOutput(t *testing.T) {
	testutil.RequireTmux(t)

	ctx := context.Background()
	sessionName := "tessariq-test-runner-pane-" + t.Name()
	t.Cleanup(func() { _ = tmux.KillSession(ctx, sessionName) })

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("printf session-output"))
	r.Session = &tmux.Starter{}
	r.SessionName = sessionName

	require.NoError(t, r.Run(ctx))

	cmd := exec.CommandContext(ctx, "tmux", "capture-pane", "-p", "-t", sessionName)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "capture tmux pane: %s", out)
	require.Contains(t, string(out), "session-output")
}

func TestRunnerIntegration_SessionReadySignaled(t *testing.T) {
	testutil.RequireTmux(t)

	ctx := context.Background()
	sessionName := "tessariq-test-ready-" + t.Name()
	t.Cleanup(func() { _ = tmux.KillSession(ctx, sessionName) })

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 0"))
	r.Session = &tmux.Starter{}
	r.SessionName = sessionName

	ready := make(chan struct{})
	r.SessionReady = ready

	require.NoError(t, r.Run(ctx))

	// Channel must be closed after successful session creation.
	select {
	case <-ready:
		// OK
	default:
		t.Fatal("SessionReady channel was not closed after real tmux session creation")
	}

	exists, err := tmux.HasSession(ctx, sessionName)
	require.NoError(t, err)
	require.True(t, exists, "tmux session should exist after runner completes")
}

func TestRunnerIntegration_CappedRunLog(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Process emits 1000 bytes; cap at 256 (includes the bracketing lines).
	r := newIntegrationRunner(dir, newShellProcess("printf '"+strings.Repeat("x", 1000)+"'"))
	r.LogCapBytes = 256

	require.NoError(t, r.Run(context.Background()))

	data, err := os.ReadFile(filepath.Join(dir, "run.log"))
	require.NoError(t, err)

	// CappedWriter allows exactly capBytes of content + the marker.
	require.Equal(t, 256+len(TruncationMarker), len(data),
		"run.log must be exactly cap + marker size")
	require.True(t, strings.HasSuffix(string(data), TruncationMarker),
		"run.log must end with truncation marker")
}
