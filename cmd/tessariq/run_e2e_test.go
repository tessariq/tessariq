//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
// builds a test Docker image with the fake agent binary, and initialises a git
// repo with a sample task file inside the container.
//
// HOME is set to /work/home so that worktree paths are under the bind mount and
// accessible to sibling containers via the Docker socket.
func setupRunEnv(t *testing.T, bin string, claudeExitCode int) *containers.RunEnv {
	return setupRunEnvForBinary(t, bin, "claude", claudeExitCode)
}

// setupRunEnvForBinary creates a RunEnv container with a fake agent binary,
// copies the tessariq binary into it, builds a test Docker image containing the
// fake binary, and initialises a git repo with a sample task file inside the
// container.
func setupRunEnvForBinary(t *testing.T, bin string, binaryName string, exitCode int) *containers.RunEnv {
	t.Helper()
	script := fmt.Sprintf("exit %d", exitCode)
	return setupRunEnvWithScript(t, bin, binaryName, script)
}

// setupRunEnvWithScript creates a RunEnv container with a custom fake agent
// script, builds a test Docker image, and initialises a git repo.
func setupRunEnvWithScript(t *testing.T, bin string, binaryName string, scriptBody string) *containers.RunEnv {
	t.Helper()

	ctx := context.Background()
	env, err := containers.StartRunEnvWithScript(ctx, t, binaryName, scriptBody)
	require.NoError(t, err)

	// Copy the tessariq binary into the bind-mounted dir.
	binData, err := os.ReadFile(bin)
	require.NoError(t, err)
	destBin := filepath.Join(env.Dir(), "tessariq")
	require.NoError(t, os.WriteFile(destBin, binData, 0o755))

	// Set HOME to /work/home so worktrees land under the bind mount.
	homeSetup := []string{
		"mkdir -p /work/home",
		"export HOME=/work/home",
	}
	for _, cmd := range homeSetup {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "home setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "home setup %q exited %d: %s", cmd, code, out)
	}

	// Build a test Docker image with the fake agent binary.
	// The image includes a tessariq user matching the reference image contract.
	buildCmds := []string{
		fmt.Sprintf(`cat > /work/Dockerfile.test <<'DEOF'
FROM alpine:latest
RUN addgroup -S tessariq && adduser -S tessariq -G tessariq -h /home/tessariq
COPY fake-agent.sh /usr/local/bin/%s
RUN chmod +x /usr/local/bin/%s
USER tessariq
WORKDIR /work
DEOF`, binaryName, binaryName),
		fmt.Sprintf(`printf '#!/bin/sh\n%s\n' > /work/fake-agent.sh && chmod +x /work/fake-agent.sh`,
			strings.ReplaceAll(scriptBody, "'", "'\\''")),
		fmt.Sprintf("docker build -t tessariq-test-agent-%s -f /work/Dockerfile.test /work", binaryName),
	}

	for _, cmd := range buildCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "image build %q: %s", cmd, out)
		require.Equal(t, 0, code, "image build %q exited %d: %s", cmd, code, out)
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "rmi", "-f", fmt.Sprintf("tessariq-test-agent-%s", binaryName)).Run()
	})

	// Create fake auth files for the agent so auth discovery succeeds.
	var authCmds []string
	switch binaryName {
	case "claude":
		authCmds = []string{
			"mkdir -p /work/home/.claude",
			`printf '{"token":"fake"}' > /work/home/.claude/.credentials.json`,
			`printf '{}' > /work/home/.claude.json`,
		}
	case "opencode":
		authCmds = []string{
			"mkdir -p /work/home/.local/share/opencode",
			`printf '{"token":"fake","provider":"https://api.example.com"}' > /work/home/.local/share/opencode/auth.json`,
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
		"cd " + repoDir + " && HOME=/work/home /work/tessariq init",
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

// runTessariq executes tessariq inside the e2e container with HOME set correctly
// and the test agent image specified via --image.
func runTessariq(t *testing.T, env *containers.RunEnv, binaryName, extraFlags string) (int, string) {
	t.Helper()
	ctx := context.Background()
	imgFlag := fmt.Sprintf("--image tessariq-test-agent-%s", binaryName)
	cmd := fmt.Sprintf("cd %s && HOME=/work/home /work/tessariq run %s %s tasks/sample.md", repoDir, imgFlag, extraFlags)
	code, output, err := env.Exec(ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err)
	return code, output
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

	code, output := runTessariq(t, env, "claude", "")
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

	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	ctx := context.Background()

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
	require.Equal(t, "custom", runtimeInfo.ImageSource, "test uses --image so source must be custom")
	require.Equal(t, "read-only", runtimeInfo.AuthMountMode)
}

func TestE2E_OpenCodeDetachedRunPrintsGuidance(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnvForBinary(t, bin, "opencode", 0)

	code, output := runTessariq(t, env, "opencode", "--agent opencode")
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

	code, output := runTessariq(t, env, "opencode", "--agent opencode")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	ctx := context.Background()

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
}

func TestE2E_OpenCodeEgressAutoFailsWhenProviderUnresolvable(t *testing.T) {
	bin := buildBinary(t)

	ctx := context.Background()
	env, err := containers.StartRunEnvForBinary(ctx, t, "opencode", 0)
	require.NoError(t, err)

	// Copy the tessariq binary.
	binData, readErr := os.ReadFile(bin)
	require.NoError(t, readErr)
	destBin := filepath.Join(env.Dir(), "tessariq")
	require.NoError(t, os.WriteFile(destBin, binData, 0o755))

	// Setup HOME under bind mount.
	setupCmds := []string{
		"mkdir -p /work/home",
	}
	for _, cmd := range setupCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "setup %q exited %d: %s", cmd, code, out)
	}

	// Create auth.json WITHOUT provider info.
	authCmds := []string{
		"mkdir -p /work/home/.local/share/opencode",
		`printf '{"token":"fake"}' > /work/home/.local/share/opencode/auth.json`,
	}
	for _, cmd := range authCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "auth setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "auth setup %q exited %d: %s", cmd, code, out)
	}

	// Init git repo.
	cmds := []string{
		"mkdir -p " + repoDir + "/tasks",
		"git init " + repoDir,
		"git -C " + repoDir + " config user.email test@test.com",
		"git -C " + repoDir + " config user.name Test",
		"printf '# Sample Task\\n\\nDo something.\\n' > " + repoDir + "/tasks/sample.md",
		"git -C " + repoDir + " add -A",
		"git -C " + repoDir + " commit -m initial",
		"cd " + repoDir + " && HOME=/work/home /work/tessariq init",
		"git -C " + repoDir + " add -A",
		"git -C " + repoDir + " commit -m 'add tessariq config'",
	}
	for _, cmd := range cmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "cmd %q failed: %s", cmd, out)
		require.Equal(t, 0, code, "cmd %q exited %d: %s", cmd, code, out)
	}

	// Run with --agent opencode (default egress=auto) — should fail before container start.
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		"cd " + repoDir + " && HOME=/work/home /work/tessariq run --agent opencode tasks/sample.md"})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "run should fail when provider unresolvable")
	require.Contains(t, output, "configure the provider")
	require.Contains(t, output, "--egress-allow")
}

func TestE2E_InitFailsWithActionableGuidanceWhenGitMissing(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	_, _, err := env.Exec(ctx, []string{"sh", "-c", "mkdir -p /work/bin-empty"})
	require.NoError(t, err)

	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && PATH=/work/bin-empty HOME=/work/home /work/tessariq init"})
	require.NoError(t, err)
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"git\" is missing or unavailable")
	require.Contains(t, output, "install or enable git, then retry")
}

func TestE2E_MountAgentConfigSetsClaudeConfigDir(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()

	// Create the config dir so --mount-agent-config can discover it.
	// Auth files are already created by setupRunEnv.

	code, output := runTessariq(t, env, "claude", "--mount-agent-config")
	require.Equal(t, 0, code, "run failed: %s", output)

	// Verify via runtime.json that the config dir was mounted.
	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	catCode, runtimeData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "runtime.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "runtime.json must exist")

	var runtimeInfo adapter.RuntimeInfo
	require.NoError(t, json.Unmarshal([]byte(runtimeData), &runtimeInfo))
	require.Equal(t, "enabled", runtimeInfo.AgentConfigMount,
		"agent config mount must be enabled")
	require.Equal(t, "mounted", runtimeInfo.AgentConfigMountStatus,
		"agent config mount status must be mounted when config dir exists")
}

func TestE2E_RunFailsWithActionableGuidanceWhenTmuxMissing(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	// Create a bin dir with only git and docker but not tmux.
	_, _, err := env.Exec(ctx, []string{"sh", "-c",
		"mkdir -p /work/bin && ln -sf $(command -v git) /work/bin/git && ln -sf $(command -v docker) /work/bin/docker"})
	require.NoError(t, err)

	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && PATH=/work/bin HOME=/work/home /work/tessariq run tasks/sample.md"})
	require.NoError(t, err)
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"tmux\" is missing or unavailable")
	require.Contains(t, output, "install or enable tmux, then retry")
	require.NotContains(t, output, "run_id: ")
	require.NotContains(t, output, "attach: tessariq attach ")
	require.NotContains(t, output, "promote: tessariq promote ")
}

func TestE2E_RunFailsWithActionableGuidanceWhenDockerMissing(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	// Create a bin dir with git and tmux but not docker.
	_, _, err := env.Exec(ctx, []string{"sh", "-c",
		"mkdir -p /work/bin && ln -sf $(command -v git) /work/bin/git && ln -sf $(command -v tmux) /work/bin/tmux"})
	require.NoError(t, err)

	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && PATH=/work/bin HOME=/work/home /work/tessariq run tasks/sample.md"})
	require.NoError(t, err)
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"docker\" is missing or unavailable")
	require.Contains(t, output, "install or enable docker, then retry")
}

func TestE2E_AgentExecutesInsideContainer(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	// Verify status.json shows success.
	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()
	catCode, statusData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "status.json must exist")

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(statusData), &status))
	require.Equal(t, "success", status["state"])
	require.Equal(t, float64(0), status["exit_code"])

	// Verify container name matches the expected pattern.
	containerName := extractField(output, "container_name")
	require.True(t, strings.HasPrefix(containerName, "tessariq-"),
		"container name must start with tessariq-")
}

func TestE2E_TmuxSessionShowsDetachedRunOutput(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", `echo detached-output`)

	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	sessionName := extractField(output, "container_name")
	require.NotEmpty(t, sessionName)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()
	time.Sleep(500 * time.Millisecond)
	hasSessionCode, hasSessionOutput, err := env.Exec(ctx, []string{"sh", "-c", "tmux has-session -t " + sessionName})
	require.NoError(t, err)
	require.Equal(t, 0, hasSessionCode, "tmux session should exist: %s", hasSessionOutput)

	logCode, runLog, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "run.log")})
	require.NoError(t, err)
	require.Equal(t, 0, logCode, "run.log must exist")
	require.Contains(t, runLog, "detached-output")
}

func TestE2E_InteractiveRunFailsWithActionableGuidance(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	code, output := runTessariq(t, env, "claude", "--interactive")
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "--interactive is not yet supported for containerized runs")
	require.NotContains(t, output, "run_id: ")
}
