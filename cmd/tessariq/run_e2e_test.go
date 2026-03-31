//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/adapter"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tessariq")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/tessariq")
	cmd.Dir = findModuleRoot(t)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", out)
	return bin
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "could not find go.mod")
		dir = parent
	}
}

const repoDir = "/work/repo"

// setupRunEnv creates a RunEnv container, copies the tessariq binary into it,
// and initialises a git repo with a sample task file inside the container.
func setupRunEnv(t *testing.T, bin string, claudeExitCode int) *containers.RunEnv {
	return setupRunEnvForBinary(t, bin, "claude", claudeExitCode)
}

// setupRunEnvForBinary creates a RunEnv container with a fake adapter binary,
// copies the tessariq binary into it, and initialises a git repo with a sample
// task file inside the container.
func setupRunEnvForBinary(t *testing.T, bin string, binaryName string, exitCode int) *containers.RunEnv {
	t.Helper()

	ctx := context.Background()
	env, err := containers.StartRunEnvForBinary(ctx, t, binaryName, exitCode)
	require.NoError(t, err)

	// Copy the tessariq binary into the bind-mounted dir.
	binData, err := os.ReadFile(bin)
	require.NoError(t, err)
	destBin := filepath.Join(env.Dir(), "tessariq")
	require.NoError(t, os.WriteFile(destBin, binData, 0o755))

	// Create fake auth files for the agent so auth discovery succeeds.
	var authCmds []string
	switch binaryName {
	case "claude":
		authCmds = []string{
			"mkdir -p /root/.claude",
			`printf '{"token":"fake"}' > /root/.claude/.credentials.json`,
			`printf '{}' > /root/.claude.json`,
		}
	case "opencode":
		authCmds = []string{
			"mkdir -p /root/.local/share/opencode",
			`printf '{"token":"fake"}' > /root/.local/share/opencode/auth.json`,
		}
	}
	for _, cmd := range authCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "auth setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "auth setup %q exited %d: %s", cmd, code, out)
	}

	// Init a git repo inside the container.
	cmds := []string{
		"mkdir -p " + repoDir + "/tasks",
		"git init " + repoDir,
		"git -C " + repoDir + " config user.email test@test.com",
		"git -C " + repoDir + " config user.name Test",
		"printf '# Sample Task\\n\\nDo something.\\n' > " + repoDir + "/tasks/sample.md",
		"git -C " + repoDir + " add -A",
		"git -C " + repoDir + " commit -m initial",
		"cd " + repoDir + " && /work/tessariq init",
		"git -C " + repoDir + " add -A",
		"git -C " + repoDir + " commit -m 'add tessariq config'",
	}

	for _, cmd := range cmds {
		code, out, err := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, err, "cmd %q failed: %s", cmd, out)
		require.Equal(t, 0, code, "cmd %q exited %d: %s", cmd, code, out)
	}

	return env
}

// extractField parses "key: value" lines from tessariq run output.
func extractField(output, key string) string {
	prefix := key + ": "
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

func TestE2E_DetachedRunPrintsGuidance(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && /work/tessariq run tasks/sample.md"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "run failed: %s", output)

	require.Contains(t, output, "run_id: ")
	require.Contains(t, output, "evidence_path: ")
	require.Contains(t, output, "workspace_path: ")
	require.Contains(t, output, "container_name: ")
	require.Contains(t, output, "attach: tessariq attach ")
	require.Contains(t, output, "promote: tessariq promote ")
}

func TestE2E_AgentAndRuntimeJSONWritten(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && /work/tessariq run tasks/sample.md"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	// Read agent.json from inside the container.
	catCode, agentData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "agent.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "agent.json must exist")

	var agentInfo adapter.AgentInfo
	require.NoError(t, json.Unmarshal([]byte(agentData), &agentInfo))
	require.Equal(t, 1, agentInfo.SchemaVersion)
	require.Equal(t, "claude-code", agentInfo.Agent)
	require.NotNil(t, agentInfo.Requested)
	require.NotNil(t, agentInfo.Applied)

	// Read runtime.json from inside the container.
	catCode, runtimeData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "runtime.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "runtime.json must exist")

	var runtimeInfo adapter.RuntimeInfo
	require.NoError(t, json.Unmarshal([]byte(runtimeData), &runtimeInfo))
	require.Equal(t, 1, runtimeInfo.SchemaVersion)
	require.NotEmpty(t, runtimeInfo.Image)
	require.Equal(t, "reference", runtimeInfo.ImageSource)
	require.Equal(t, "read-only", runtimeInfo.AuthMountMode)
}

func TestE2E_OpenCodeDetachedRunPrintsGuidance(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnvForBinary(t, bin, "opencode", 0)

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && /work/tessariq run --agent opencode tasks/sample.md"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "run failed: %s", output)

	require.Contains(t, output, "run_id: ")
	require.Contains(t, output, "evidence_path: ")
	require.Contains(t, output, "workspace_path: ")
	require.Contains(t, output, "container_name: ")
	require.Contains(t, output, "attach: tessariq attach ")
	require.Contains(t, output, "promote: tessariq promote ")
}

func TestE2E_OpenCodeAgentAndRuntimeJSONWritten(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnvForBinary(t, bin, "opencode", 0)

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && /work/tessariq run --agent opencode tasks/sample.md"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	// Read agent.json from inside the container.
	catCode, agentData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "agent.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "agent.json must exist")

	var agentInfo adapter.AgentInfo
	require.NoError(t, json.Unmarshal([]byte(agentData), &agentInfo))
	require.Equal(t, 1, agentInfo.SchemaVersion)
	require.Equal(t, "opencode", agentInfo.Agent)
	require.NotNil(t, agentInfo.Requested)
	require.NotNil(t, agentInfo.Applied)
	require.False(t, agentInfo.Applied["interactive"],
		"opencode does not apply interactive")

	// Read runtime.json from inside the container.
	catCode, runtimeData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "runtime.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "runtime.json must exist")

	var runtimeInfo adapter.RuntimeInfo
	require.NoError(t, json.Unmarshal([]byte(runtimeData), &runtimeInfo))
	require.Equal(t, 1, runtimeInfo.SchemaVersion)
	require.NotEmpty(t, runtimeInfo.Image)
	require.Equal(t, "reference", runtimeInfo.ImageSource)
}

func TestE2E_InitFailsWithActionableGuidanceWhenGitMissing(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	_, _, err := env.Exec(ctx, []string{"sh", "-c", "mkdir -p /work/bin-empty"})
	require.NoError(t, err)

	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && PATH=/work/bin-empty /work/tessariq init"})
	require.NoError(t, err)
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"git\" is missing or unavailable")
	require.Contains(t, output, "install or enable git, then retry")
}

func TestE2E_MountAgentConfigSetsClaudeConfigDir(t *testing.T) {
	bin := buildBinary(t)

	ctx := context.Background()
	// Use a custom claude script that writes CLAUDE_CONFIG_DIR to a file.
	env, err := containers.StartRunEnvWithScript(ctx, t, "claude",
		`printf '%s' "$CLAUDE_CONFIG_DIR" > /tmp/claude_config_dir.txt`)
	require.NoError(t, err)

	// Copy the tessariq binary into the bind-mounted dir.
	binData, readErr := os.ReadFile(bin)
	require.NoError(t, readErr)
	destBin := filepath.Join(env.Dir(), "tessariq")
	require.NoError(t, os.WriteFile(destBin, binData, 0o755))

	// Init a git repo inside the container.
	cmds := []string{
		"mkdir -p " + repoDir + "/tasks",
		"git init " + repoDir,
		"git -C " + repoDir + " config user.email test@test.com",
		"git -C " + repoDir + " config user.name Test",
		"printf '# Sample Task\\n\\nDo something.\\n' > " + repoDir + "/tasks/sample.md",
		"git -C " + repoDir + " add -A",
		"git -C " + repoDir + " commit -m initial",
		"cd " + repoDir + " && /work/tessariq init",
		"git -C " + repoDir + " add -A",
		"git -C " + repoDir + " commit -m 'add tessariq config'",
	}
	for _, cmd := range cmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "cmd %q failed: %s", cmd, out)
		require.Equal(t, 0, code, "cmd %q exited %d: %s", cmd, code, out)
	}

	// Create auth files and config dir so --mount-agent-config can discover them.
	authSetup := []string{
		"mkdir -p /root/.claude",
		`printf '{"token":"fake"}' > /root/.claude/.credentials.json`,
		`printf '{}' > /root/.claude.json`,
	}
	for _, cmd := range authSetup {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "auth setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "auth setup %q exited %d: %s", cmd, code, out)
	}

	// Run tessariq with --mount-agent-config.
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		"cd " + repoDir + " && /work/tessariq run --mount-agent-config tasks/sample.md"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "run failed: %s", output)

	// Verify the fake claude saw CLAUDE_CONFIG_DIR.
	catCode, configDir, err := env.Exec(ctx, []string{"cat", "/tmp/claude_config_dir.txt"})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "config dir file must exist")
	require.Equal(t, "/home/tessariq/.claude", configDir,
		"CLAUDE_CONFIG_DIR must point to the container-home .claude dir")
}

func TestE2E_RunFailsWithActionableGuidanceWhenTmuxMissing(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	_, _, err := env.Exec(ctx, []string{"sh", "-c", "mkdir -p /work/bin && ln -sf $(command -v git) /work/bin/git"})
	require.NoError(t, err)

	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && PATH=/work/bin /work/tessariq run tasks/sample.md"})
	require.NoError(t, err)
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"tmux\" is missing or unavailable")
	require.Contains(t, output, "install or enable tmux, then retry")
	require.NotContains(t, output, "run_id: ")
	require.NotContains(t, output, "attach: tessariq attach ")
	require.NotContains(t, output, "promote: tessariq promote ")
}
