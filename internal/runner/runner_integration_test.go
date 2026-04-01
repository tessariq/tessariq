//go:build integration

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/tmux"
)

// shellProcess runs a shell command as the ProcessRunner for integration tests.
type shellProcess struct {
	command string
	cmd     *exec.Cmd
	stdout  *os.File
	stderr  *os.File
}

func newShellProcess(command string) *shellProcess {
	return &shellProcess{command: command}
}

func (s *shellProcess) Start(ctx context.Context) error {
	s.cmd = exec.CommandContext(ctx, "sh", "-c", s.command)
	s.cmd.Stdout = s.stdout
	s.cmd.Stderr = s.stderr
	return s.cmd.Start()
}

func (s *shellProcess) Wait() (int, error) {
	err := s.cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

func (s *shellProcess) Signal(sig os.Signal) error {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Signal(sig)
	}
	return nil
}

func (s *shellProcess) SetOutput(stdout, stderr *os.File) {
	s.stdout = stdout
	s.stderr = stderr
}

func newIntegrationRunner(dir string, proc ProcessRunner) *Runner {
	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	return &Runner{
		RunID:       "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		EvidenceDir: dir,
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

	require.NoError(t, r.Run(context.Background()))

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

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	_, err = os.Stat(filepath.Join(dir, "timeout.flag"))
	require.NoError(t, err, "timeout.flag must exist")
}

func TestRunnerIntegration_EvidenceDurability(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 1"))

	require.NoError(t, r.Run(context.Background()))

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

	require.NoError(t, r.Run(context.Background()))

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

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
}

func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}
}

func TestRunnerIntegration_TmuxSessionCreated(t *testing.T) {
	t.Parallel()
	skipIfNoTmux(t)

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
	t.Parallel()
	skipIfNoTmux(t)

	ctx := context.Background()
	sessionName := "tessariq-test-runner-fail-" + t.Name()
	t.Cleanup(func() { _ = tmux.KillSession(ctx, sessionName) })

	dir := t.TempDir()
	r := newIntegrationRunner(dir, newShellProcess("exit 1"))
	r.Session = &tmux.Starter{}
	r.SessionName = sessionName

	require.NoError(t, r.Run(ctx))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)

	exists, err := tmux.HasSession(ctx, sessionName)
	require.NoError(t, err)
	require.True(t, exists, "tmux session should persist even when process fails")
}

func TestRunnerIntegration_TmuxSessionShowsRunLogOutput(t *testing.T) {
	t.Parallel()
	skipIfNoTmux(t)

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
