package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
)

// fakeProcess simulates a process for unit testing.
type fakeProcess struct {
	exitCode     int
	waitCh       chan struct{} // closed when Wait should return
	stdoutWriter io.Writer
	stderrWriter io.Writer
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

func (f *fakeProcess) SetOutputWriter(stdout, stderr io.Writer) {
	f.stdoutWriter = stdout
	f.stderrWriter = stderr
}

// signalRecordingProcess records signals and controls exit behavior per signal type.
// - exitOnTERM: if true, process exits immediately on SIGTERM; otherwise ignores it.
// - Always exits on SIGKILL.
type signalRecordingProcess struct {
	mu         sync.Mutex
	signals    []os.Signal
	exitCode   int
	exitOnTERM bool
	waitCh     chan struct{}
	stdout     *os.File
	stderr     *os.File
	onSignal   func(os.Signal) // optional hook called inside Signal before acting
}

func newSignalRecordingProcess(exitCode int, exitOnTERM bool) *signalRecordingProcess {
	return &signalRecordingProcess{
		exitCode:   exitCode,
		exitOnTERM: exitOnTERM,
		waitCh:     make(chan struct{}),
	}
}

func (s *signalRecordingProcess) Start(_ context.Context) error { return nil }

func (s *signalRecordingProcess) Wait() (int, error) {
	<-s.waitCh
	return s.exitCode, nil
}

func (s *signalRecordingProcess) Signal(sig os.Signal) error {
	s.mu.Lock()
	s.signals = append(s.signals, sig)
	if s.onSignal != nil {
		s.onSignal(sig)
	}
	s.mu.Unlock()

	switch sig {
	case syscall.SIGTERM:
		if s.exitOnTERM {
			select {
			case <-s.waitCh:
			default:
				close(s.waitCh)
			}
		}
		// If !exitOnTERM, do nothing — process stays alive.
	case syscall.SIGKILL, os.Kill:
		select {
		case <-s.waitCh:
		default:
			close(s.waitCh)
		}
	}
	return nil
}

func (s *signalRecordingProcess) SetOutput(stdout, stderr *os.File) {
	s.stdout = stdout
	s.stderr = stderr
}

func (s *signalRecordingProcess) Signals() []os.Signal {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]os.Signal, len(s.signals))
	copy(result, s.signals)
	return result
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

// fakeSession simulates a SessionStarter for unit testing.
type fakeSession struct {
	startErr    error
	startCalled bool
	sessionName string
	command     []string
}

func (f *fakeSession) StartSession(_ context.Context, name string, command []string) error {
	f.startCalled = true
	f.sessionName = name
	f.command = append([]string(nil), command...)
	return f.startErr
}

func TestRunner_SessionStarted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	sess := &fakeSession{}
	r.Session = sess
	r.SessionName = "tessariq-TESTID"

	require.NoError(t, r.Run(context.Background()))
	require.True(t, sess.startCalled)
	require.Equal(t, "tessariq-TESTID", sess.sessionName)
	require.Equal(t, []string{"tail", "-n", "+1", "-f", filepath.Join(dir, "run.log")}, sess.command)
}

func TestRunner_SessionStartFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Session = &fakeSession{startErr: errors.New("tmux not available")}
	r.SessionName = "tessariq-TESTID"

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
}

func TestRunner_SessionStartFailure_StatusExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Session = &fakeSession{startErr: errors.New("session error")}
	r.SessionName = "tessariq-TESTID"

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.NotEmpty(t, s.StartedAt)
	require.NotEmpty(t, s.FinishedAt)
}

func TestRunner_NilSession_DoesNotPanic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Session = nil

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State)
}

func TestRunner_SessionNamePassedThrough(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	sess := &fakeSession{}
	r.Session = sess
	r.SessionName = "custom-session-name"

	require.NoError(t, r.Run(context.Background()))
	require.Equal(t, "custom-session-name", sess.sessionName)
}

func TestRunner_ConfiguresProcessOutputWhenSupported(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newFakeProcess(0)
	r := newTestRunner(dir, proc)

	require.NoError(t, r.Run(context.Background()))
	require.NotNil(t, proc.stdoutWriter)
	require.NotNil(t, proc.stderrWriter)

	// Both stdout and stderr should be the same CappedWriter for run.log.
	cw, ok := proc.stdoutWriter.(*CappedWriter)
	require.True(t, ok, "stdout writer should be a *CappedWriter")
	require.Same(t, cw, proc.stderrWriter, "stdout and stderr should share the same writer")
}

func TestRunner_InteractiveSessionCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Config.Interactive = true
	r.ContainerName = "tessariq-RUN123"
	sess := &fakeSession{}
	r.Session = sess
	r.SessionName = "tessariq-RUN123"

	require.NoError(t, r.Run(context.Background()))
	require.True(t, sess.startCalled)
	require.Equal(t, []string{"docker", "attach", "tessariq-RUN123"}, sess.command)
}

func TestRunner_InteractiveSuccessPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newFakeProcess(0)
	r := newTestRunner(dir, proc)
	r.Config.Interactive = true
	r.ContainerName = "tessariq-RUN123"

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State)
	require.Equal(t, 0, s.ExitCode)
	require.False(t, s.TimedOut)
}

func TestRunner_InteractiveFailedProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newFakeProcess(7)
	r := newTestRunner(dir, proc)
	r.Config.Interactive = true
	r.ContainerName = "tessariq-RUN123"

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
	require.Equal(t, 7, s.ExitCode)
}

func TestRunner_InteractiveTimeout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Use a blocking process that won't exit on its own.
	proc := newBlockingProcess(0)
	r := newTestRunner(dir, proc)
	r.Config.Interactive = true
	r.Config.Timeout = 50 * time.Millisecond
	r.Config.Grace = 10 * time.Millisecond
	r.ContainerName = "tessariq-RUN123"
	// Use real clock for timeout (no idle detection since fakeProcess doesn't produce output).
	r.Clock = nil

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)
}

func TestRunner_EmptySessionName_SkipsSessionStart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	sess := &fakeSession{}
	r.Session = sess
	r.SessionName = ""

	require.NoError(t, r.Run(context.Background()))
	require.False(t, sess.startCalled)
}

func TestRunner_TimeoutSendsSIGTERMBeforeSIGKILL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newSignalRecordingProcess(0, false) // ignores SIGTERM
	r := newTestRunner(dir, proc)
	r.Config.Timeout = 50 * time.Millisecond
	r.Config.Grace = 30 * time.Millisecond

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	signals := proc.Signals()
	require.Len(t, signals, 2, "expected two signals: SIGTERM then SIGKILL")
	require.Equal(t, syscall.SIGTERM, signals[0], "first signal must be SIGTERM")
	require.Equal(t, os.Kill, signals[1], "second signal must be SIGKILL")
}

func TestRunner_TimeoutNoSIGKILLWhenProcessExitsAfterSIGTERM(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newSignalRecordingProcess(0, true) // exits on SIGTERM
	r := newTestRunner(dir, proc)
	r.Config.Timeout = 50 * time.Millisecond
	r.Config.Grace = 100 * time.Millisecond

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	signals := proc.Signals()
	require.Len(t, signals, 1, "expected only SIGTERM, no SIGKILL")
	require.Equal(t, syscall.SIGTERM, signals[0], "only signal must be SIGTERM")
}

func TestRunner_TimeoutFlagWrittenBeforeFirstSignal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newSignalRecordingProcess(0, true) // exits on SIGTERM

	flagExistedAtSignalTime := false
	proc.onSignal = func(sig os.Signal) {
		if sig == syscall.SIGTERM {
			_, err := os.Stat(filepath.Join(dir, "timeout.flag"))
			flagExistedAtSignalTime = (err == nil)
		}
	}

	r := newTestRunner(dir, proc)
	r.Config.Timeout = 50 * time.Millisecond
	r.Config.Grace = 100 * time.Millisecond

	require.NoError(t, r.Run(context.Background()))
	require.True(t, flagExistedAtSignalTime, "timeout.flag must exist before first signal is sent")
}

func TestRunner_InteractiveTimeoutSendsSIGTERMFirst(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newSignalRecordingProcess(0, false) // ignores SIGTERM
	r := newTestRunner(dir, proc)
	r.Config.Interactive = true
	r.Config.Timeout = 50 * time.Millisecond
	r.Config.Grace = 30 * time.Millisecond
	r.ContainerName = "tessariq-RUN123"
	r.Clock = nil // real clock for activity timer

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	signals := proc.Signals()
	require.Len(t, signals, 2, "expected two signals: SIGTERM then SIGKILL")
	require.Equal(t, syscall.SIGTERM, signals[0], "first signal must be SIGTERM")
	require.Equal(t, os.Kill, signals[1], "second signal must be SIGKILL")
}
