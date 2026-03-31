package adapter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/authmount"
	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/proxy"
	"github.com/tessariq/tessariq/internal/run"
)

func TestNewProcess_ClaudeCode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "implement feature", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil)

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
	ap, err := NewProcess(cfg, "implement feature", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil)

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
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil)

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
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil)

	require.NoError(t, err)
	require.Equal(t, "custom", ap.RuntimeInfo.ImageSource)
	require.Equal(t, "my-registry/custom:v1", ap.RuntimeInfo.Image)
}

func TestNewProcess_ReturnsContainerProcess(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "task", "run-42", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil)

	require.NoError(t, err)
	_, ok := ap.Process.(*container.Process)
	require.True(t, ok, "process should be a *container.Process")
}

func TestNewProcess_UnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "unknown-agent"
	cfg.TaskPath = "specs/task.md"
	_, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown-agent")
}

func TestNewProcess_ClaudeCode_WithEnvVars(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	envVars := map[string]string{"CLAUDE_CONFIG_DIR": "/home/tessariq/.claude"}
	ap, err := NewProcess(cfg, "implement feature", "run-1", "/wt", "/ev",
		nil, nil, "enabled", "mounted", envVars, nil)

	require.NoError(t, err)
	require.NotNil(t, ap)
	require.Equal(t, "claude-code", ap.AgentInfo.Agent)
	require.Equal(t, "enabled", ap.RuntimeInfo.AgentConfigMount)
	require.Equal(t, "mounted", ap.RuntimeInfo.AgentConfigMountStatus)
}

func TestNewProcess_AuthMountCount(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	authMounts := []authmount.MountSpec{
		{HostPath: "/h/cred", ContainerPath: "/c/cred", ReadOnly: true},
		{HostPath: "/h/cfg", ContainerPath: "/c/cfg", ReadOnly: true},
	}
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		authMounts, nil, "disabled", "disabled", nil, nil)

	require.NoError(t, err)
	require.Equal(t, 2, ap.RuntimeInfo.AuthMountCount)
}

func TestNewProcess_WithProxyEnv(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	pEnv := &proxy.ProxyEnv{
		ProxyAddr:   "http://tessariq-squid-run1:3128",
		NetworkName: "tessariq-net-run1",
	}
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, pEnv)

	require.NoError(t, err)
	require.NotNil(t, ap)

	proc := ap.Process.(*container.Process)
	_ = proc // container process created with network name
}

func TestNewProcess_NilProxyEnv(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil)

	require.NoError(t, err)
	require.NotNil(t, ap)
}

func TestMergeEnvVars_BothNil(t *testing.T) {
	t.Parallel()
	require.Nil(t, mergeEnvVars(nil, nil))
}

func TestMergeEnvVars_AOnly(t *testing.T) {
	t.Parallel()
	m := mergeEnvVars(map[string]string{"A": "1"}, nil)
	require.Equal(t, map[string]string{"A": "1"}, m)
}

func TestMergeEnvVars_BOverrides(t *testing.T) {
	t.Parallel()
	m := mergeEnvVars(
		map[string]string{"A": "1", "B": "2"},
		map[string]string{"B": "3", "C": "4"},
	)
	require.Equal(t, "1", m["A"])
	require.Equal(t, "3", m["B"])
	require.Equal(t, "4", m["C"])
}
