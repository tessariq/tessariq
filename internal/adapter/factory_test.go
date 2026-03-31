package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
)

func TestNewProcess_ClaudeCode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "implement feature", 0)

	require.NoError(t, err)
	require.NotNil(t, ap)
	require.NotNil(t, ap.Process)
	require.Equal(t, "claude-code", ap.AgentInfo.Agent)
	require.Equal(t, 1, ap.AgentInfo.SchemaVersion)
	require.Equal(t, 1, ap.RuntimeInfo.SchemaVersion)
	require.Equal(t, "reference", ap.RuntimeInfo.ImageSource)
}

func TestNewProcess_OpenCode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "opencode"
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "implement feature", 0)

	require.NoError(t, err)
	require.NotNil(t, ap)
	require.NotNil(t, ap.Process)
	require.Equal(t, "opencode", ap.AgentInfo.Agent)
	require.Equal(t, 1, ap.AgentInfo.SchemaVersion)
	require.False(t, ap.AgentInfo.Applied["interactive"],
		"opencode does not apply interactive")
	require.Equal(t, "reference", ap.RuntimeInfo.ImageSource)
}

func TestNewProcess_OpenCodeWithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "opencode"
	cfg.TaskPath = "specs/task.md"
	cfg.Model = "sonnet"
	ap, err := NewProcess(cfg, "task", 0)

	require.NoError(t, err)
	require.Equal(t, "sonnet", ap.AgentInfo.Requested["model"])
	require.False(t, ap.AgentInfo.Applied["model"],
		"opencode does not apply model")
}

func TestNewProcess_CustomImageSource(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	cfg.Image = "my-registry/custom:v1"
	ap, err := NewProcess(cfg, "task", 0)

	require.NoError(t, err)
	require.Equal(t, "custom", ap.RuntimeInfo.ImageSource)
	require.Equal(t, "my-registry/custom:v1", ap.RuntimeInfo.Image)
}

func TestNewProcess_BinaryNotFoundMessageConsistency(t *testing.T) {
	// Not parallel: t.Setenv modifies process environment.
	t.Setenv("PATH", t.TempDir())

	for _, agent := range []string{"claude-code", "opencode"} {
		t.Run(agent, func(t *testing.T) {
			cfg := run.DefaultConfig()
			cfg.Agent = agent
			cfg.TaskPath = "specs/task.md"
			ap, err := NewProcess(cfg, "task", 0)
			require.NoError(t, err)

			startErr := ap.Process.Start(context.Background())
			require.Error(t, startErr)
			require.Contains(t, startErr.Error(), "adapter binary")
			require.Contains(t, startErr.Error(), "container image")
			require.Contains(t, startErr.Error(), "--image")
		})
	}
}

func TestNewProcess_UnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "unknown-agent"
	cfg.TaskPath = "specs/task.md"
	_, err := NewProcess(cfg, "task", 0)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown-agent")
}
