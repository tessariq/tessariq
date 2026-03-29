package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
)

// fakeProcess simulates a process for unit testing.
type fakeProcess struct {
	exitCode int
	waitCh   chan struct{} // closed when Wait should return
}

func newFakeProcess(exitCode int) *fakeProcess {
	ch := make(chan struct{})
	close(ch) // immediate return
	return &fakeProcess{exitCode: exitCode, waitCh: ch}
}

func newBlockingProcess(exitCode int) *fakeProcess {
	return &fakeProcess{exitCode: exitCode, waitCh: make(chan struct{})}
}

func (f *fakeProcess) Start(_ context.Context) error { return nil }

func (f *fakeProcess) Wait() (int, error) {
	<-f.waitCh
	return f.exitCode, nil
}

func (f *fakeProcess) Signal(_ os.Signal) error {
	select {
	case <-f.waitCh:
	default:
		close(f.waitCh)
	}
	return nil
}

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func newTestRunner(dir string, proc ProcessRunner) *Runner {
	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	return &Runner{
		RunID:       "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		EvidenceDir: dir,
		Config:      cfg,
		Process:     proc,
		Clock:       fixedClock(time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)),
	}
}

func TestRunner_SuccessPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State)
	require.Equal(t, 0, s.ExitCode)
	require.False(t, s.TimedOut)
	require.NotEmpty(t, s.StartedAt)
	require.NotEmpty(t, s.FinishedAt)
}

func TestRunner_FailedProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(1))

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
	require.Equal(t, 1, s.ExitCode)
	require.False(t, s.TimedOut)
}

func TestRunner_NilProcess_SuccessPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, nil)

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State)
}

func TestRunner_TimeoutPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newBlockingProcess(0)
	r := newTestRunner(dir, proc)
	r.Config.Timeout = 50 * time.Millisecond
	r.Config.Grace = 10 * time.Millisecond

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	_, err = os.Stat(filepath.Join(dir, "timeout.flag"))
	require.NoError(t, err, "timeout.flag should exist")
}

func TestRunner_PreHookFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Config.Pre = []string{"false"}

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
}

func TestRunner_VerifyHookFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Config.Verify = []string{"false"}

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
}

func TestRunner_WritesInitialStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))

	require.NoError(t, r.Run(context.Background()))

	// After run, status should be terminal (overwritten initial)
	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.True(t, s.State.IsTerminal())
}

func TestRunner_LogFilesExist(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))

	require.NoError(t, r.Run(context.Background()))

	_, err := os.Stat(filepath.Join(dir, "run.log"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
}

func TestRunner_LogFilesExistOnFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(1))

	require.NoError(t, r.Run(context.Background()))

	_, err := os.Stat(filepath.Join(dir, "run.log"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
}

func TestRunner_StatusExistsOnPreHookFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Config.Pre = []string{"false"}

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
	require.NotEmpty(t, s.StartedAt)
	require.NotEmpty(t, s.FinishedAt)
}

func TestRunner_SchemaVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, 1, s.SchemaVersion)
}
