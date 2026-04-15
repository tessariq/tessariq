//go:build integration

package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// noBudgetIntegration disables hook budget enforcement in integration tests.
var noBudgetIntegration = time.Time{}

func TestRunPreHooksIntegration_RealProcess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	var buf bytes.Buffer

	results, err := RunPreHooks(ctx, noBudgetIntegration, 0, []string{"echo hello", "echo world"}, dir, &buf)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, 0, results[0].ExitCode)
	require.Equal(t, 0, results[1].ExitCode)
	require.Contains(t, buf.String(), "hello")
	require.Contains(t, buf.String(), "world")
}

func TestRunPreHooksIntegration_FailureStopsExecution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	var buf bytes.Buffer

	cmds := []string{"echo first", "exit 42", "echo third"}
	results, err := RunPreHooks(ctx, noBudgetIntegration, 0, cmds, dir, &buf)
	require.Error(t, err)
	require.Len(t, results, 2)
	require.Equal(t, 0, results[0].ExitCode)
	require.Equal(t, 42, results[1].ExitCode)
	require.Contains(t, buf.String(), "first")
	require.NotContains(t, buf.String(), "third")
}

func TestRunVerifyHooksIntegration_AllRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	var buf bytes.Buffer

	cmds := []string{"echo a", "exit 1", "echo c"}
	results, err := RunVerifyHooks(ctx, noBudgetIntegration, 0, cmds, dir, &buf)
	require.Error(t, err)
	require.Len(t, results, 3)
	require.Contains(t, buf.String(), "a")
	require.Contains(t, buf.String(), "c")
}

func TestRunPreHooksIntegration_WorkDirUsed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	var buf bytes.Buffer

	results, err := RunPreHooks(ctx, noBudgetIntegration, 0, []string{"pwd"}, dir, &buf)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Contains(t, buf.String(), dir)
}

// TASK-091: integration coverage proving real shell children are actually
// killed when the run --timeout budget is exhausted, not just marked
// cancelled in Go state. The marker file is written by a backgrounded
// shell that should never get a chance to run because the parent shell is
// SIGKILL'd before the sleep returns.
func TestRunPreHooksIntegration_TimedOutShellChildIsKilled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	marker := filepath.Join(dir, "post.marker")

	// The shell sleeps for 30s, then writes a marker. With a 100ms deadline
	// and a 200ms grace, the entire process group must be torn down before
	// the marker is written.
	cmdLine := "sleep 30 && echo done > " + marker
	deadline := time.Now().Add(100 * time.Millisecond)
	grace := 200 * time.Millisecond

	start := time.Now()
	results, err := RunPreHooks(ctx, deadline, grace, []string{cmdLine}, dir, &bytes.Buffer{})
	elapsed := time.Since(start)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrHookTimeout)
	require.Len(t, results, 1)
	require.True(t, results[0].TimedOut)
	require.Less(t, elapsed, 5*time.Second)

	// Wait a tiny bit longer than the original sleep would have taken to
	// reach the marker write — if any descendant survived, the marker would
	// appear during this window.
	time.Sleep(200 * time.Millisecond)
	_, statErr := os.Stat(marker)
	require.True(t, os.IsNotExist(statErr),
		"shell descendant survived signal escalation; marker exists at %s", marker)
}

func TestRunPreHooksIntegration_RelativePathFromRepoRoot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repoRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "Makefile"), []byte("all:"), 0o644))

	var buf bytes.Buffer
	results, err := RunPreHooks(ctx, noBudgetIntegration, 0, []string{"ls Makefile"}, repoRoot, &buf)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, 0, results[0].ExitCode)
	require.Contains(t, buf.String(), "Makefile")
}
