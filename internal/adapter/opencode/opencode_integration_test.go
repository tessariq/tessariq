//go:build integration

package opencode_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

func TestOpenCodeIntegration_BinaryNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, err := containers.StartAdapterEnvForBinary(ctx, t, "opencode", -1)
	require.NoError(t, err)

	code, _, err := env.Exec(ctx, []string{"opencode", "test"})
	require.NoError(t, err)
	require.Equal(t, 127, code, "missing binary should exit 127 (command not found)")
}

func TestOpenCodeIntegration_ProcessCrash(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, err := containers.StartAdapterEnvForBinary(ctx, t, "opencode", 7)
	require.NoError(t, err)

	code, _, err := env.Exec(ctx, []string{"opencode", "crash"})
	require.NoError(t, err)
	require.Equal(t, 7, code, "exit code must reflect the crash")
}

func TestOpenCodeIntegration_SuccessfulInvocation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, err := containers.StartAdapterEnvForBinary(ctx, t, "opencode", 0)
	require.NoError(t, err)

	code, _, err := env.Exec(ctx, []string{"opencode", "ok"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
}

func TestOpenCodeIntegration_ProcessCrashNoOutput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, err := containers.StartAdapterEnvWithScriptForBinary(ctx, t, "opencode", "kill -9 $$")
	require.NoError(t, err)

	code, _, err := env.Exec(ctx, []string{"opencode"})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "crashed process should have non-zero exit code")
}
