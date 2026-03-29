package runner

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunPreHooks_Empty(t *testing.T) {
	t.Parallel()

	results, err := RunPreHooks(context.Background(), nil, t.TempDir(), &bytes.Buffer{})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestRunPreHooks_SingleSuccess(t *testing.T) {
	t.Parallel()

	results, err := RunPreHooks(context.Background(), []string{"true"}, t.TempDir(), &bytes.Buffer{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, 0, results[0].ExitCode)
}

func TestRunPreHooks_HaltsOnFirstFailure(t *testing.T) {
	t.Parallel()

	cmds := []string{"true", "false", "true"}
	results, err := RunPreHooks(context.Background(), cmds, t.TempDir(), &bytes.Buffer{})
	require.Error(t, err)
	require.Len(t, results, 2, "should stop after second command fails")
	require.Equal(t, 0, results[0].ExitCode)
	require.NotEqual(t, 0, results[1].ExitCode)
}

func TestRunPreHooks_OrderPreserved(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmds := []string{"echo first", "echo second"}
	results, err := RunPreHooks(context.Background(), cmds, t.TempDir(), &buf)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Contains(t, buf.String(), "first")
	require.Contains(t, buf.String(), "second")
}

func TestRunVerifyHooks_Empty(t *testing.T) {
	t.Parallel()

	results, err := RunVerifyHooks(context.Background(), nil, t.TempDir(), &bytes.Buffer{})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestRunVerifyHooks_RunsAll(t *testing.T) {
	t.Parallel()

	cmds := []string{"true", "false", "true"}
	results, err := RunVerifyHooks(context.Background(), cmds, t.TempDir(), &bytes.Buffer{})
	require.Error(t, err, "should report failure")
	require.Len(t, results, 3, "all three commands should run")
	require.Equal(t, 0, results[0].ExitCode)
	require.NotEqual(t, 0, results[1].ExitCode)
	require.Equal(t, 0, results[2].ExitCode)
}

func TestRunVerifyHooks_AllSucceed(t *testing.T) {
	t.Parallel()

	results, err := RunVerifyHooks(context.Background(), []string{"true", "true"}, t.TempDir(), &bytes.Buffer{})
	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestRunPreHooks_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RunPreHooks(ctx, []string{"sleep 10"}, t.TempDir(), &bytes.Buffer{})
	require.Error(t, err)
}
