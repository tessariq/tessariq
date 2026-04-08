package adapter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/authmount"
	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/proxy"
	"github.com/tessariq/tessariq/internal/run"
)

func TestNewAgent_ClaudeCode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a, err := NewAgent(cfg, "task content", nil)

	require.NoError(t, err)
	require.Equal(t, "claude-code", a.Name())
	require.Equal(t, "claude", a.BinaryName())
	require.NotEmpty(t, a.Image())
	require.NotEmpty(t, a.Args())
}

func TestNewAgent_OpenCode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "opencode"
	a, err := NewAgent(cfg, "task content", nil)

	require.NoError(t, err)
	require.Equal(t, "opencode", a.Name())
	require.Equal(t, "opencode", a.BinaryName())
	require.NotEmpty(t, a.Image())
	require.NotEmpty(t, a.Args())
}

func TestNewAgent_OpenCodeWithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "opencode"
	cfg.Model = "anthropic/claude-sonnet-4-20250514"
	a, err := NewAgent(cfg, "task", nil)

	require.NoError(t, err)
	require.Contains(t, a.Args(), "--model")
	require.Contains(t, a.Args(), "anthropic/claude-sonnet-4-20250514")
	require.True(t, a.Supported()["model"])
}

func TestNewAgent_UnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "unknown"
	_, err := NewAgent(cfg, "task", nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown")
}

func TestNewProcess_ClaudeCode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "implement feature", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

	require.NoError(t, err)
	require.NotNil(t, ap)
	require.NotNil(t, ap.Process)
	require.Equal(t, "claude-code", ap.AgentInfo.Agent)
	require.Equal(t, 1, ap.AgentInfo.SchemaVersion)
	require.Equal(t, 1, ap.RuntimeInfo.SchemaVersion)
	require.Equal(t, "reference", ap.RuntimeInfo.ImageSource)
	require.Equal(t, "claude", ap.BinaryName)
}

func TestNewProcess_OpenCode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "opencode"
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "implement feature", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

	require.NoError(t, err)
	require.NotNil(t, ap)
	require.NotNil(t, ap.Process)
	require.Equal(t, "opencode", ap.AgentInfo.Agent)
	require.Equal(t, 1, ap.AgentInfo.SchemaVersion)
	require.True(t, ap.AgentInfo.Supported["interactive"],
		"supported is a capability flag: adapter supports interactive")
	require.Equal(t, "reference", ap.RuntimeInfo.ImageSource)
	require.Equal(t, "opencode", ap.BinaryName)
}

func TestNewProcess_OpenCodeWithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "opencode"
	cfg.TaskPath = "specs/task.md"
	cfg.Model = "sonnet"
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

	require.NoError(t, err)
	require.Equal(t, "sonnet", ap.AgentInfo.Requested["model"])
	require.True(t, ap.AgentInfo.Supported["model"],
		"opencode forwards model as-is")
}

func TestNewProcess_OpenCodeInteractive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Agent = "opencode"
	cfg.TaskPath = "specs/task.md"
	cfg.Interactive = true
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

	require.NoError(t, err)
	require.Equal(t, true, ap.AgentInfo.Requested["interactive"],
		"interactive must be recorded as requested")
	require.True(t, ap.AgentInfo.Supported["interactive"],
		"supported is a capability flag: adapter supports interactive")
}

func TestNewProcess_CustomImageSource(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	cfg.Image = "my-registry/custom:v1"
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

	require.NoError(t, err)
	require.Equal(t, "custom", ap.RuntimeInfo.ImageSource)
	require.Equal(t, "my-registry/custom:v1", ap.RuntimeInfo.Image)
}

func TestNewProcess_ReturnsContainerProcess(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "task", "run-42", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

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
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown-agent")
}

func TestNewProcess_ClaudeCode_WithEnvVars(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	envVars := map[string]string{"CLAUDE_CONFIG_DIR": "/home/tessariq/.claude"}
	ap, err := NewProcess(cfg, "implement feature", "run-1", "/wt", "/ev",
		nil, nil, "enabled", "mounted", envVars, nil, "open", UpdateResult{})

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
		authMounts, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

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
		nil, nil, "disabled", "disabled", nil, pEnv, "proxy", UpdateResult{})

	require.NoError(t, err)
	require.NotNil(t, ap)

	proc := ap.Process.(*container.Process)
	require.Equal(t, "tessariq-net-run1", proc.NetworkName())
}

func TestNewProcess_NilProxyEnv(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

	require.NoError(t, err)
	require.NotNil(t, ap)

	proc := ap.Process.(*container.Process)
	require.Empty(t, proc.NetworkName(), "open mode should not set network name")
}

func TestNewProcess_EgressNone_SetsNetworkNone(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "none", UpdateResult{})

	require.NoError(t, err)

	proc := ap.Process.(*container.Process)
	require.Equal(t, "none", proc.NetworkName(),
		"egress none must set Docker network to none")
}

func TestNewProcess_EgressOpen_NoNetworkOverride(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

	require.NoError(t, err)

	proc := ap.Process.(*container.Process)
	require.Empty(t, proc.NetworkName(),
		"open mode must use default bridge network")
}

func TestNewProcess_EgressProxy_UsesProxyNetwork(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	pEnv := &proxy.ProxyEnv{
		ProxyAddr:   "http://tessariq-squid-run1:3128",
		NetworkName: "tessariq-net-run1",
	}
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, pEnv, "proxy", UpdateResult{})

	require.NoError(t, err)

	proc := ap.Process.(*container.Process)
	require.Equal(t, "tessariq-net-run1", proc.NetworkName(),
		"proxy mode must use proxy network")
}

func TestWritableDirsForFileMounts_FileLevelMount(t *testing.T) {
	t.Parallel()
	mounts := []authmount.MountSpec{
		{ContainerPath: "/home/tessariq/.claude/.credentials.json", ReadOnly: true},
	}
	dirs := writableDirsForFileMounts(mounts, nil)
	require.Equal(t, []string{"/home/tessariq/.claude"}, dirs)
}

func TestWritableDirsForFileMounts_TopLevelSkipped(t *testing.T) {
	t.Parallel()
	mounts := []authmount.MountSpec{
		{ContainerPath: "/home/tessariq/.claude.json", ReadOnly: true},
	}
	dirs := writableDirsForFileMounts(mounts, nil)
	require.Empty(t, dirs, "top-level home mounts should not produce writable dirs")
}

func TestWritableDirsForFileMounts_Deduplication(t *testing.T) {
	t.Parallel()
	mounts := []authmount.MountSpec{
		{ContainerPath: "/home/tessariq/.claude/.credentials.json", ReadOnly: true},
		{ContainerPath: "/home/tessariq/.claude/settings.json", ReadOnly: true},
	}
	dirs := writableDirsForFileMounts(mounts, nil)
	require.Equal(t, []string{"/home/tessariq/.claude"}, dirs)
}

func TestWritableDirsForFileMounts_Empty(t *testing.T) {
	t.Parallel()
	dirs := writableDirsForFileMounts(nil, nil)
	require.Empty(t, dirs)
}

func TestWritableDirsForFileMounts_DeeplyNestedMount(t *testing.T) {
	t.Parallel()
	mounts := []authmount.MountSpec{
		{ContainerPath: "/home/tessariq/.local/share/opencode/auth.json", ReadOnly: true},
	}
	dirs := writableDirsForFileMounts(mounts, nil)
	require.ElementsMatch(t, []string{
		"/home/tessariq/.local/share/opencode",
		"/home/tessariq/.local/share",
		"/home/tessariq/.local",
	}, dirs)
}

func TestWritableDirsForFileMounts_DeeplyNestedWithConfigOverlap(t *testing.T) {
	t.Parallel()
	authMounts := []authmount.MountSpec{
		{ContainerPath: "/home/tessariq/.local/share/opencode/auth.json", ReadOnly: true},
	}
	configMounts := []authmount.MountSpec{
		{ContainerPath: "/home/tessariq/.local/share", ReadOnly: true},
	}
	dirs := writableDirsForFileMounts(authMounts, configMounts)
	require.ElementsMatch(t, []string{
		"/home/tessariq/.local/share/opencode",
		"/home/tessariq/.local",
	}, dirs, "dir covered by config mount must be excluded but ancestors and children still included")
}

func TestWritableDirsForFileMounts_ConfigMountOverlap(t *testing.T) {
	t.Parallel()
	authMounts := []authmount.MountSpec{
		{ContainerPath: "/home/tessariq/.claude/.credentials.json", ReadOnly: true},
	}
	configMounts := []authmount.MountSpec{
		{ContainerPath: "/home/tessariq/.claude", ReadOnly: true},
	}
	dirs := writableDirsForFileMounts(authMounts, configMounts)
	require.Empty(t, dirs, "dir covered by config mount must not become a writable dir")
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

func TestNewProcess_WithSuccessfulUpdate_RuntimeInfo(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	update := UpdateResult{
		Attempted:     true,
		Success:       true,
		CachedVersion: "2.3.0",
		BakedVersion:  "2.1.92",
		ElapsedMs:     4200,
		CacheHostPath: "/home/user/.tessariq/agent-cache/claude-code",
	}
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", update)

	require.NoError(t, err)
	require.NotNil(t, ap.RuntimeInfo.AgentUpdate)
	require.True(t, ap.RuntimeInfo.AgentUpdate.Success)
	require.Equal(t, "2.3.0", ap.RuntimeInfo.AgentUpdate.CachedVersion)
}

func TestNewProcess_WithFailedUpdate_RuntimeInfo(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	update := UpdateResult{
		Attempted:    true,
		Success:      false,
		BakedVersion: "2.1.92",
		ElapsedMs:    1500,
		Error:        "npm install failed",
	}
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", update)

	require.NoError(t, err)
	require.NotNil(t, ap.RuntimeInfo.AgentUpdate)
	require.False(t, ap.RuntimeInfo.AgentUpdate.Success)
	require.Equal(t, "npm install failed", ap.RuntimeInfo.AgentUpdate.Error)
}

func TestNewProcess_RuntimeInfo_IncludesAgentUpdate(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	update := UpdateResult{
		Attempted:     true,
		Success:       true,
		CachedVersion: "2.3.0",
		BakedVersion:  "2.1.92",
		ElapsedMs:     4200,
		CacheHostPath: "/cache/path",
	}
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", update)

	require.NoError(t, err)
	require.NotNil(t, ap.RuntimeInfo.AgentUpdate)
	require.True(t, ap.RuntimeInfo.AgentUpdate.Attempted)
	require.True(t, ap.RuntimeInfo.AgentUpdate.Success)
	require.Equal(t, "2.3.0", ap.RuntimeInfo.AgentUpdate.CachedVersion)
	require.Equal(t, "2.1.92", ap.RuntimeInfo.AgentUpdate.BakedVersion)
	require.Equal(t, int64(4200), ap.RuntimeInfo.AgentUpdate.ElapsedMs)
}

func TestNewProcess_RuntimeInfo_NilAgentUpdateWhenNotAttempted(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	ap, err := NewProcess(cfg, "task", "run-1", "/wt", "/ev",
		nil, nil, "disabled", "disabled", nil, nil, "open", UpdateResult{})

	require.NoError(t, err)
	require.Nil(t, ap.RuntimeInfo.AgentUpdate)
}
