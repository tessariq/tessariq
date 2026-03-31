//go:build integration

package claudecode_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

func TestClaudeCodeIntegration_BinaryNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, err := containers.StartAgentEnv(ctx, t, -1)
	require.NoError(t, err)

	code, _, err := env.Exec(ctx, []string{"claude", "--print", "test"})
	require.NoError(t, err)
	require.Equal(t, 127, code, "missing binary should exit 127 (command not found)")
}

func TestClaudeCodeIntegration_ProcessCrash(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, err := containers.StartAgentEnv(ctx, t, 7)
	require.NoError(t, err)

	code, _, err := env.Exec(ctx, []string{"claude", "--print", "crash"})
	require.NoError(t, err)
	require.Equal(t, 7, code, "exit code must reflect the crash")
}

func TestClaudeCodeIntegration_SuccessfulInvocation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, err := containers.StartAgentEnv(ctx, t, 0)
	require.NoError(t, err)

	code, _, err := env.Exec(ctx, []string{"claude", "--print", "ok"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
}

func TestClaudeCodeIntegration_ProcessCrashNoOutput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, err := containers.StartAgentEnvWithScript(ctx, t, "kill -9 $$")
	require.NoError(t, err)

	code, _, err := env.Exec(ctx, []string{"claude"})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "crashed process should have non-zero exit code")
}

func TestClaudeCodeIntegration_EnvVarVisibleInProcess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, err := containers.StartAgentEnvWithScript(ctx, t, `echo "CONFIG_DIR=$CLAUDE_CONFIG_DIR"`)
	require.NoError(t, err)

	// Set the env var inside the container and invoke the fake claude binary.
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "CLAUDE_CONFIG_DIR=/home/tessariq/.claude claude"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Contains(t, output, "CONFIG_DIR=/home/tessariq/.claude")
}
