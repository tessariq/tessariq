package adapter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
)

func TestNewProcess_ClaudeCode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "implement feature")

	require.NoError(t, err)
	require.NotNil(t, ap)
	require.NotNil(t, ap.Process)
	require.Equal(t, "claude-code", ap.Metadata.Adapter)
	require.Equal(t, 1, ap.Metadata.SchemaVersion)
}

func TestNewProcess_OpenCodeNotImplemented(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "opencode"
	cfg.TaskPath = "specs/task.md"
	_, err := NewProcess(cfg, "task")

	require.Error(t, err)
	require.Contains(t, err.Error(), "opencode")
}

func TestNewProcess_UnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "unknown-agent"
	cfg.TaskPath = "specs/task.md"
	_, err := NewProcess(cfg, "task")

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown-agent")
}
