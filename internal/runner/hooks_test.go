package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// noBudget is a sentinel deadline meaning "disable timeout enforcement".
var noBudget = time.Time{}

func TestRunPreHooks_Empty(t *testing.T) {
	t.Parallel()

	results, err := RunPreHooks(context.Background(), noBudget, 0, nil, t.TempDir(), &bytes.Buffer{})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestRunPreHooks_SingleSuccess(t *testing.T) {
	t.Parallel()

	results, err := RunPreHooks(context.Background(), noBudget, 0, []string{"true"}, t.TempDir(), &bytes.Buffer{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, 0, results[0].ExitCode)
	require.False(t, results[0].TimedOut)
}

func TestRunPreHooks_HaltsOnFirstFailure(t *testing.T) {
	t.Parallel()

	cmds := []string{"true", "false", "true"}
	results, err := RunPreHooks(context.Background(), noBudget, 0, cmds, t.TempDir(), &bytes.Buffer{})
	require.Error(t, err)
	require.Len(t, results, 2, "should stop after second command fails")
	require.Equal(t, 0, results[0].ExitCode)
	require.NotEqual(t, 0, results[1].ExitCode)
}

func TestRunPreHooks_OrderPreserved(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmds := []string{"echo first", "echo second"}
	results, err := RunPreHooks(context.Background(), noBudget, 0, cmds, t.TempDir(), &buf)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Contains(t, buf.String(), "first")
	require.Contains(t, buf.String(), "second")
}

func TestRunVerifyHooks_Empty(t *testing.T) {
	t.Parallel()

	results, err := RunVerifyHooks(context.Background(), noBudget, 0, nil, t.TempDir(), &bytes.Buffer{})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestRunVerifyHooks_RunsAll(t *testing.T) {
	t.Parallel()

	cmds := []string{"true", "false", "true"}
	results, err := RunVerifyHooks(context.Background(), noBudget, 0, cmds, t.TempDir(), &bytes.Buffer{})
	require.Error(t, err, "should report failure")
	require.Len(t, results, 3, "all three commands should run")
	require.Equal(t, 0, results[0].ExitCode)
	require.NotEqual(t, 0, results[1].ExitCode)
	require.Equal(t, 0, results[2].ExitCode)
}

func TestRunVerifyHooks_AllSucceed(t *testing.T) {
	t.Parallel()

	results, err := RunVerifyHooks(context.Background(), noBudget, 0, []string{"true", "true"}, t.TempDir(), &bytes.Buffer{})
	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestRunPreHooks_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RunPreHooks(ctx, noBudget, 50*time.Millisecond, []string{"sleep 10"}, t.TempDir(), &bytes.Buffer{})
	require.Error(t, err)
}

// TASK-091: pre-hook exceeds the budget => timeout-tagged result + sentinel error.
func TestRunPreHooks_TimesOutOnBudget(t *testing.T) {
	t.Parallel()

	deadline := time.Now().Add(80 * time.Millisecond)
	start := time.Now()

	results, err := RunPreHooks(context.Background(), deadline, 50*time.Millisecond,
		[]string{"sleep 5"}, t.TempDir(), &bytes.Buffer{})
	elapsed := time.Since(start)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrHookTimeout)
	var hte *HookTimeoutError
	require.ErrorAs(t, err, &hte)
	require.Equal(t, HookPhasePre, hte.Phase)
	require.Equal(t, 0, hte.Index)
	require.Equal(t, "sleep 5", hte.Command)

	require.Len(t, results, 1)
	require.True(t, results[0].TimedOut, "result should be marked TimedOut")
	require.Less(t, elapsed, 2*time.Second, "should not take anywhere near the 5s sleep")
}

func TestRunPreHooks_TimeoutHaltsRemaining(t *testing.T) {
	t.Parallel()

	deadline := time.Now().Add(60 * time.Millisecond)
	cmds := []string{"sleep 5", "echo should-not-run"}

	var buf bytes.Buffer
	results, err := RunPreHooks(context.Background(), deadline, 40*time.Millisecond,
		cmds, t.TempDir(), &buf)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrHookTimeout)
	require.Len(t, results, 1, "remaining commands must not run after a timeout")
	require.NotContains(t, buf.String(), "should-not-run")
}

func TestRunVerifyHooks_TimesOutOnBudget(t *testing.T) {
	t.Parallel()

	deadline := time.Now().Add(80 * time.Millisecond)
	cmds := []string{"sleep 5", "echo should-not-run"}

	var buf bytes.Buffer
	results, err := RunVerifyHooks(context.Background(), deadline, 40*time.Millisecond,
		cmds, t.TempDir(), &buf)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrHookTimeout)
	var hte *HookTimeoutError
	require.ErrorAs(t, err, &hte)
	require.Equal(t, HookPhaseVerify, hte.Phase)
	require.Equal(t, 0, hte.Index)

	require.Len(t, results, 1, "verify hooks must halt after a timeout")
	require.NotContains(t, buf.String(), "should-not-run")
}

func TestRunPreHooks_SIGTERMHonoredBeforeKill(t *testing.T) {
	t.Parallel()

	deadline := time.Now().Add(60 * time.Millisecond)
	grace := 500 * time.Millisecond

	// sh traps SIGTERM and exits voluntarily with code 42 — SIGKILL must
	// never fire because the process leaves during the grace window.
	cmd := `trap 'exit 42' TERM; sleep 10`
	start := time.Now()

	results, err := RunPreHooks(context.Background(), deadline, grace,
		[]string{cmd}, t.TempDir(), &bytes.Buffer{})
	elapsed := time.Since(start)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrHookTimeout)
	require.Len(t, results, 1)
	require.Equal(t, 42, results[0].ExitCode, "TERM-trapped exit code must be preserved")
	require.True(t, results[0].TimedOut)
	require.Less(t, elapsed, grace+300*time.Millisecond,
		"should exit well inside the grace window and never reach SIGKILL")
}

func TestRunPreHooks_UnderBudgetUnaffected(t *testing.T) {
	t.Parallel()

	deadline := time.Now().Add(5 * time.Second)
	results, err := RunPreHooks(context.Background(), deadline, 500*time.Millisecond,
		[]string{"true", "echo ok"}, t.TempDir(), &bytes.Buffer{})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, 0, results[0].ExitCode)
	require.Equal(t, 0, results[1].ExitCode)
	require.False(t, results[0].TimedOut)
	require.False(t, results[1].TimedOut)
}

// BUG-060: deadline already expired before the shell is spawned. runHook
// must refuse to start the command and tag the result as TimedOut so the
// caller maps it to StateTimeout + timeout.flag. Previously cmd.Start() ran
// unconditionally and a fast command could execute and be recorded as a
// regular success, breaking the --timeout guarantee.
func TestRunPreHooks_AlreadyExpiredDeadline_SkipsExecution(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	sentinel := filepath.Join(workDir, "sentinel")

	deadline := time.Now().Add(-time.Hour)
	// touch-then-sleep: without the pre-Start guard the shell gets a chance
	// to create the sentinel before SIGTERM arrives for the sleep. With the
	// guard, runHook must refuse to Start at all and the file stays absent.
	cmd := fmt.Sprintf("touch %q; sleep 10", sentinel)

	results, err := RunPreHooks(context.Background(), deadline, 50*time.Millisecond,
		[]string{cmd}, workDir, &bytes.Buffer{})

	require.Error(t, err)
	require.ErrorIs(t, err, ErrHookTimeout)
	require.Len(t, results, 1)
	require.True(t, results[0].TimedOut, "expired deadline must tag result TimedOut")

	_, statErr := os.Stat(sentinel)
	require.True(t, os.IsNotExist(statErr),
		"hook body must not execute when deadline already elapsed (sentinel=%q, err=%v)", sentinel, statErr)
}

func TestRunVerifyHooks_AlreadyExpiredDeadline_SkipsExecution(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	sentinel := filepath.Join(workDir, "sentinel-verify")

	deadline := time.Now().Add(-time.Hour)
	cmd := fmt.Sprintf("touch %q; sleep 10", sentinel)

	results, err := RunVerifyHooks(context.Background(), deadline, 50*time.Millisecond,
		[]string{cmd}, workDir, &bytes.Buffer{})

	require.Error(t, err)
	require.ErrorIs(t, err, ErrHookTimeout)
	require.Len(t, results, 1)
	require.True(t, results[0].TimedOut)

	_, statErr := os.Stat(sentinel)
	require.True(t, os.IsNotExist(statErr))
}

// Canceled context (not a deadline) must not be reported as TimedOut — it is
// a caller-initiated abort and maps to a different terminal state.
func TestRunPreHooks_AlreadyCanceledContext_NotTimedOut(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, err := RunPreHooks(ctx, noBudget, 50*time.Millisecond,
		[]string{"true"}, t.TempDir(), &bytes.Buffer{})

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrHookTimeout)
	require.Len(t, results, 1)
	require.False(t, results[0].TimedOut, "canceled ctx is not a deadline overrun")
}

// Guard against unwrap drift for ErrHookTimeout.
func TestHookTimeoutErrorUnwrap(t *testing.T) {
	t.Parallel()

	err := &HookTimeoutError{Phase: HookPhasePre, Index: 0, Command: "sleep 1"}
	require.True(t, errors.Is(err, ErrHookTimeout))
	require.Contains(t, err.Error(), "pre-hook")
	require.Contains(t, err.Error(), "sleep 1")
}
