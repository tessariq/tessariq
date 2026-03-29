//go:build integration

package runner

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunPreHooksIntegration_RealProcess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	var buf bytes.Buffer

	results, err := RunPreHooks(ctx, []string{"echo hello", "echo world"}, dir, &buf)
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
	results, err := RunPreHooks(ctx, cmds, dir, &buf)
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
	results, err := RunVerifyHooks(ctx, cmds, dir, &buf)
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

	results, err := RunPreHooks(ctx, []string{"pwd"}, dir, &buf)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Contains(t, buf.String(), dir)
}
