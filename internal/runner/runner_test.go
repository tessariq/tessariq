package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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

type directOutputProcess struct {
	mu             sync.Mutex
	stdout         *os.File
	stderr         *os.File
	stopCh         chan struct{}
	stoppedCh      chan struct{}
	startedWrite   chan struct{}
	stopLogsCh     chan struct{}
	stoppedWriting chan struct{}
	onWrite        func()
	exitCode       int
}

func newDirectOutputProcess(exitCode int) *directOutputProcess {
	return &directOutputProcess{
		stopCh:         make(chan struct{}),
		stoppedCh:      make(chan struct{}),
		startedWrite:   make(chan struct{}),
		stopLogsCh:     make(chan struct{}),
		stoppedWriting: make(chan struct{}),
		exitCode:       exitCode,
	}
}

func (p *directOutputProcess) Start(_ context.Context) error {
	go func() {
		defer close(p.stoppedCh)
		close(p.startedWrite)
		chunk := []byte(strings.Repeat("x", 256))
		for {
			select {
			case <-p.stopCh:
				return
			case <-p.stopLogsCh:
				// StopLogStream was called by the cap monitor. Signal
				// test observers that this goroutine has stopped
				// writing new chunks, then block until Signal closes
				// stopCh so Wait can observe a terminal state.
				close(p.stoppedWriting)
				<-p.stopCh
				return
			default:
			}

			p.mu.Lock()
			stdout := p.stdout
			onWrite := p.onWrite
			p.mu.Unlock()
			if stdout != nil {
				_, _ = stdout.Write(chunk)
				if onWrite != nil {
					onWrite()
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	return nil
}

func (p *directOutputProcess) Wait() (int, error) {
	<-p.stoppedCh
	return p.exitCode, nil
}

func (p *directOutputProcess) Signal(_ os.Signal) error {
	select {
	case <-p.stopCh:
	default:
		close(p.stopCh)
	}
	return nil
}

func (p *directOutputProcess) SetOutput(stdout, stderr *os.File) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stdout = stdout
	p.stderr = stderr
}

func (p *directOutputProcess) StopLogStream() error {
	select {
	case <-p.stopLogsCh:
	default:
		close(p.stopLogsCh)
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
		RepoRoot:    dir,
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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)
	require.Equal(t, 1, termErr.ExitCode)

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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateTimeout, termErr.State)

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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
}

func TestRunner_VerifyHookFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Config.Verify = []string{"false"}

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)

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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)

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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)

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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)

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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)

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
	require.Equal(t, []string{"docker", "attach", "tessariq-RUN123"}, sess.command,
		"interactive mode must use docker attach in tmux session for detach/reattach support")
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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)
	require.Equal(t, 7, termErr.ExitCode)

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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateTimeout, termErr.State)

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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateTimeout, termErr.State)

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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateTimeout, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	signals := proc.Signals()
	require.Len(t, signals, 1, "expected only SIGTERM, no SIGKILL")
	require.Equal(t, syscall.SIGTERM, signals[0], "only signal must be SIGTERM")
}

func TestRunner_DetachedContextCancellationMarksInterrupted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newSignalRecordingProcess(130, true)
	r := newTestRunner(dir, proc)
	r.Config.Timeout = time.Minute
	r.Config.Grace = 100 * time.Millisecond

	ctx, cancel := context.WithCancelCause(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel(SignalCause(syscall.SIGINT))
	}()

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(ctx), &termErr)
	require.Equal(t, StateInterrupted, termErr.State)
	require.Equal(t, 130, termErr.ExitCode)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateInterrupted, s.State)
	require.Equal(t, 130, s.ExitCode)
	require.False(t, s.TimedOut)

	signals := proc.Signals()
	require.Len(t, signals, 1)
	require.Equal(t, syscall.SIGTERM, signals[0], "runner must gracefully terminate the process before finalizing")

	logData, err := os.ReadFile(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
	require.Contains(t, string(logData), "context cancelled")
	if strings.Contains(string(logData), "timeout reached") {
		t.Fatal("context cancellation must not be treated as a timeout")
	}
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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
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

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateTimeout, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateTimeout, s.State)
	require.True(t, s.TimedOut)

	signals := proc.Signals()
	require.Len(t, signals, 2, "expected two signals: SIGTERM then SIGKILL")
	require.Equal(t, syscall.SIGTERM, signals[0], "first signal must be SIGTERM")
	require.Equal(t, os.Kill, signals[1], "second signal must be SIGKILL")
}

func TestRunner_PreHookRunsFromRepoRoot(t *testing.T) {
	t.Parallel()

	evidenceDir := t.TempDir()
	repoRoot := t.TempDir()

	// Place a marker file in the repo root only.
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "Makefile"), []byte("all:"), 0o644))

	r := newTestRunner(evidenceDir, newFakeProcess(0))
	r.RepoRoot = repoRoot
	r.Config.Pre = []string{"ls Makefile"}

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(evidenceDir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State, "pre-hook should find Makefile in repo root")
}

func TestRunner_SessionReadySignaled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	sess := &fakeSession{}
	r.Session = sess
	r.SessionName = "tessariq-TESTID"

	ready := make(chan struct{})
	r.SessionReady = ready

	require.NoError(t, r.Run(context.Background()))

	// Channel must be closed after successful session creation.
	select {
	case <-ready:
		// OK — channel was closed
	default:
		t.Fatal("SessionReady channel was not closed after successful session creation")
	}
}

func TestRunner_SessionReadyNil_NoPanic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	sess := &fakeSession{}
	r.Session = sess
	r.SessionName = "tessariq-TESTID"
	r.SessionReady = nil

	require.NoError(t, r.Run(context.Background()))
	require.True(t, sess.startCalled)
}

func TestRunner_SessionReadyNotSignaledOnFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Session = &fakeSession{startErr: errors.New("tmux failed")}
	r.SessionName = "tessariq-TESTID"

	ready := make(chan struct{})
	r.SessionReady = ready

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)

	// Channel must NOT be closed when session creation fails.
	select {
	case <-ready:
		t.Fatal("SessionReady channel should not be closed when session creation fails")
	default:
		// OK — channel remains open
	}
}

// orderTracker records the order of Start and StartSession calls.
type orderTracker struct {
	mu    sync.Mutex
	calls []string
}

func (o *orderTracker) record(name string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls = append(o.calls, name)
}

func (o *orderTracker) order() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	result := make([]string, len(o.calls))
	copy(result, o.calls)
	return result
}

// orderTrackingProcess wraps fakeProcess and records Start calls.
type orderTrackingProcess struct {
	*fakeProcess
	tracker *orderTracker
}

func (o *orderTrackingProcess) Start(ctx context.Context) error {
	o.tracker.record("Start")
	return o.fakeProcess.Start(ctx)
}

// orderTrackingSession wraps fakeSession and records StartSession calls.
type orderTrackingSession struct {
	fakeSession
	tracker *orderTracker
}

func (o *orderTrackingSession) StartSession(ctx context.Context, name string, command []string) error {
	o.tracker.record("StartSession")
	return o.fakeSession.StartSession(ctx, name, command)
}

// readyCheckingSession checks whether SessionReady was closed before StartSession.
type readyCheckingSession struct {
	fakeSession
	readyCh            chan struct{}
	readyBeforeSession bool
}

func (s *readyCheckingSession) StartSession(ctx context.Context, name string, command []string) error {
	select {
	case <-s.readyCh:
		s.readyBeforeSession = true
	default:
		s.readyBeforeSession = false
	}
	return s.fakeSession.StartSession(ctx, name, command)
}

func TestRunner_InteractiveSessionReadyFiredAfterTmux(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Config.Interactive = true
	r.ContainerName = "tessariq-RUN123"

	ready := make(chan struct{})
	r.SessionReady = ready

	sess := &readyCheckingSession{readyCh: ready}
	r.Session = sess
	r.SessionName = "tessariq-RUN123"

	require.NoError(t, r.Run(context.Background()))
	require.True(t, sess.startCalled, "tmux session should still be created")
	require.False(t, sess.readyBeforeSession,
		"SessionReady must fire AFTER StartSession so tmux attach can find the session")
}

func TestRunner_InteractiveSessionCreatedAfterProcessStart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tracker := &orderTracker{}
	proc := &orderTrackingProcess{fakeProcess: newFakeProcess(0), tracker: tracker}
	r := newTestRunner(dir, proc)
	r.Config.Interactive = true
	r.ContainerName = "tessariq-RUN123"
	sess := &orderTrackingSession{tracker: tracker}
	r.Session = sess
	r.SessionName = "tessariq-RUN123"

	require.NoError(t, r.Run(context.Background()))
	require.True(t, sess.startCalled)

	calls := tracker.order()
	require.Equal(t, []string{"Start", "StartSession"}, calls,
		"interactive mode must start process before creating tmux session")
}

func TestRunner_InteractiveSessionReadySignaled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Config.Interactive = true
	r.ContainerName = "tessariq-RUN123"
	sess := &fakeSession{}
	r.Session = sess
	r.SessionName = "tessariq-RUN123"

	ready := make(chan struct{})
	r.SessionReady = ready

	require.NoError(t, r.Run(context.Background()))

	select {
	case <-ready:
		// OK — channel was closed
	default:
		t.Fatal("SessionReady channel was not closed after interactive session creation")
	}
}

func TestRunner_InteractiveSessionFailure_NonFatal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Config.Interactive = true
	r.ContainerName = "tessariq-RUN123"
	r.Session = &fakeSession{startErr: errors.New("tmux not available")}
	r.SessionName = "tessariq-RUN123"

	require.NoError(t, r.Run(context.Background()),
		"interactive mode must not fail when tmux session creation fails")

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State)
}

func TestRunner_InteractiveSessionReadySignaledDespiteTmuxFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.Config.Interactive = true
	r.ContainerName = "tessariq-RUN123"
	r.Session = &fakeSession{startErr: errors.New("tmux failed")}
	r.SessionName = "tessariq-RUN123"

	ready := make(chan struct{})
	r.SessionReady = ready

	require.NoError(t, r.Run(context.Background()))

	select {
	case <-ready:
		// OK — SessionReady fires after process start, regardless of tmux outcome
	default:
		t.Fatal("SessionReady must be closed even when tmux session creation fails")
	}
}

func TestRunner_DetachedDirectOutputCapsLogBeforeProcessExit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	proc := newDirectOutputProcess(0)
	r := newTestRunner(dir, proc)
	// Generous timeout — the test completes its assertions well before
	// this. The timeout only matters for the final StateTimeout check.
	r.Config.Timeout = 2 * time.Second
	r.Config.Grace = 20 * time.Millisecond
	r.LogCapBytes = 512
	r.Clock = nil

	runDone := make(chan error, 1)
	go func() {
		runDone <- r.Run(context.Background())
	}()

	// Synchronize with the start of the writer goroutine.
	select {
	case <-proc.startedWrite:
	case <-time.After(2 * time.Second):
		t.Fatal("writer never started")
	}

	// Wait until the cap monitor has (a) truncated run.log at least once
	// and (b) called StopLogStream on the fake process, which causes the
	// writer goroutine to stop issuing new chunks. This explicit sync
	// point eliminates a sparse-hole race where the writer's stale fd
	// offset extended the file between monitor ticks and the previous
	// polling loop occasionally snapshotted the file with the marker at
	// the end but a size well beyond the cap.
	select {
	case <-proc.stoppedWriting:
	case <-time.After(3 * time.Second):
		t.Fatal("writer never stopped; cap monitor did not kick in before process exit")
	}

	// The runner must still be alive — this test verifies that the cap
	// happens BEFORE process exit, not after.
	select {
	case err := <-runDone:
		t.Fatalf("runner exited before cap took effect: %v", err)
	default:
	}

	// With the writer quiescent, run.log stabilizes to the capped state
	// within at most one monitor tick (25 ms). Poll briefly for the final
	// deterministic state — by this point no further writes can extend
	// the file, so the length invariant holds.
	logPath := filepath.Join(dir, "run.log")
	require.Eventuallyf(t, func() bool {
		data, err := os.ReadFile(logPath)
		if err != nil {
			return false
		}
		if !strings.HasSuffix(string(data), TruncationMarker) {
			return false
		}
		return int64(len(data)) <= r.LogCapBytes+int64(len(TruncationMarker))
	}, 2*time.Second, 10*time.Millisecond,
		"run.log must stabilize to <= %d bytes ending in the truncation marker after the writer stops",
		r.LogCapBytes+int64(len(TruncationMarker)))

	// Process must ultimately reach StateTimeout.
	var termErr *TerminalStateError
	require.ErrorAs(t, <-runDone, &termErr)
	require.Equal(t, StateTimeout, termErr.State)
}

func TestRunner_NonInteractiveSessionStillCreatedEarly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tracker := &orderTracker{}
	proc := &orderTrackingProcess{fakeProcess: newFakeProcess(0), tracker: tracker}
	r := newTestRunner(dir, proc)
	// Interactive is false by default.
	sess := &orderTrackingSession{tracker: tracker}
	r.Session = sess
	r.SessionName = "tessariq-TESTID"

	require.NoError(t, r.Run(context.Background()))
	require.True(t, sess.startCalled)

	calls := tracker.order()
	require.Equal(t, []string{"StartSession", "Start"}, calls,
		"non-interactive mode must create tmux session before starting process")
}

func TestRunner_DiffArtifactWriterFailure_EscalatesToFailed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	r.DiffArtifactWriter = func(_ context.Context, _ string) error {
		return errors.New("boom")
	}

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr,
		"diff-artifact writer failure after a success process must escalate to TerminalStateError")
	require.Equal(t, StateFailed, termErr.State)

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State,
		"status.json must reflect non-success when diff artifacts cannot be committed")
}

func TestRunner_DiffArtifactWriterSuccess_StaysSuccessful(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(0))
	called := false
	r.DiffArtifactWriter = func(_ context.Context, evidenceDir string) error {
		called = true
		require.Equal(t, dir, evidenceDir)
		return nil
	}

	require.NoError(t, r.Run(context.Background()))
	require.True(t, called, "diff-artifact writer should be invoked during the runner lifecycle")

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State)
}

func TestRunner_DiffArtifactWriter_InvokedOnProcessFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(7))
	called := false
	r.DiffArtifactWriter = func(_ context.Context, evidenceDir string) error {
		called = true
		require.Equal(t, dir, evidenceDir)
		return nil
	}

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)
	require.Equal(t, 7, termErr.ExitCode)
	require.True(t, called,
		"diff-artifact writer must run on non-success terminal states so promote gets diff evidence")
}

func TestRunner_DiffArtifactWriter_FailureOnFailedProcessPreservesExitCode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newTestRunner(dir, newFakeProcess(7))
	r.DiffArtifactWriter = func(_ context.Context, _ string) error {
		return errors.New("diff write boom")
	}

	var termErr *TerminalStateError
	require.ErrorAs(t, r.Run(context.Background()), &termErr)
	require.Equal(t, StateFailed, termErr.State)
	require.Equal(t, 7, termErr.ExitCode,
		"diff-write failure must not clobber the original process exit code")

	s, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, StateFailed, s.State)
	require.Equal(t, 7, s.ExitCode)
}

func TestRunner_VerifyHookRunsFromRepoRoot(t *testing.T) {
	t.Parallel()

	evidenceDir := t.TempDir()
	repoRoot := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "Makefile"), []byte("all:"), 0o644))

	r := newTestRunner(evidenceDir, newFakeProcess(0))
	r.RepoRoot = repoRoot
	r.Config.Verify = []string{"ls Makefile"}

	require.NoError(t, r.Run(context.Background()))

	s, err := ReadStatus(evidenceDir)
	require.NoError(t, err)
	require.Equal(t, StateSuccess, s.State, "verify-hook should find Makefile in repo root")
}
