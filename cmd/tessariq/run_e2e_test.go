//go:build e2e

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/adapter"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

// sharedBinaryDir holds the temp directory for the shared binary so TestMain
// can clean it up after all tests complete.
var sharedBinaryDir string

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedBinaryDir != "" {
		os.RemoveAll(sharedBinaryDir)
	}
	os.Exit(code)
}

var (
	sharedBinary     string
	sharedBinaryOnce sync.Once
	sharedBinaryErr  error
)

// buildBinary builds the tessariq CLI binary once and returns the path. The
// binary is shared across all tests in the package to avoid rebuilding 23 times.
func buildBinary(t *testing.T) string {
	t.Helper()
	sharedBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "tessariq-e2e-*")
		if err != nil {
			sharedBinaryErr = err
			return
		}
		sharedBinaryDir = dir
		bin := filepath.Join(dir, "tessariq")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/tessariq")
		cmd.Dir = findModuleRoot()
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			sharedBinaryErr = fmt.Errorf("build failed: %s: %w", out, err)
			return
		}
		sharedBinary = bin
	})
	require.NoError(t, sharedBinaryErr, "shared binary build")
	return sharedBinary
}

func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if dir == parent {
			panic("could not find go.mod")
		}
		dir = parent
	}
}

// testImageTag returns a short, Docker-safe suffix derived from t.Name() for
// unique-per-test Docker image names. This prevents races when tests run in
// parallel and build their own test images.
func testImageTag(t *testing.T) string {
	t.Helper()
	h := sha256.Sum256([]byte(t.Name()))
	return hex.EncodeToString(h[:6])
}

// setupRunEnv creates a RunEnv container, copies the tessariq binary into it,
// builds a test Docker image with the fake agent binary, and initialises a git
// repo with a sample task file inside the container.
//
// HOME is set to <hostDir>/home so that worktree, evidence, and auth paths
// are host-absolute and accessible to sibling containers via the Docker socket.
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
//
// Paths for HOME and the repo use env.Dir() (the host temp dir) so that
// tessariq-computed mount paths are host-absolute and work with the Docker
// socket sibling container pattern.
func setupRunEnvWithScript(t *testing.T, bin string, binaryName string, scriptBody string) *containers.RunEnv {
	t.Helper()

	ctx := context.Background()
	env, err := containers.StartRunEnvWithScript(ctx, t, binaryName, scriptBody)
	require.NoError(t, err)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")

	// Copy the tessariq binary into the bind-mounted dir.
	binData, err := os.ReadFile(bin)
	require.NoError(t, err)
	destBin := filepath.Join(hostDir, "tessariq")
	require.NoError(t, os.WriteFile(destBin, binData, 0o755))

	// Set HOME to a host-absolute path so worktrees and auth paths
	// are valid on the host Docker daemon for sibling container mounts.
	homeSetup := []string{
		fmt.Sprintf("mkdir -p %s", homeDir),
	}
	for _, cmd := range homeSetup {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "home setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "home setup %q exited %d: %s", cmd, code, out)
	}

	// Build a test Docker image with the fake agent binary.
	// docker-cli reads from the container filesystem, so /work paths are fine.
	imgName := fmt.Sprintf("tessariq-test-agent-%s-%s", binaryName, testImageTag(t))
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
		fmt.Sprintf("docker build -t %s -f /work/Dockerfile.test /work", imgName),
	}

	for _, cmd := range buildCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "image build %q: %s", cmd, out)
		require.Equal(t, 0, code, "image build %q exited %d: %s", cmd, code, out)
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "rmi", "-f", imgName).Run()
	})

	// Create fake auth files for the agent so auth discovery succeeds.
	var authCmds []string
	switch binaryName {
	case "claude":
		authCmds = []string{
			fmt.Sprintf("mkdir -p %s/.claude", homeDir),
			fmt.Sprintf(`printf '{"token":"fake"}' > %s/.claude/.credentials.json`, homeDir),
			fmt.Sprintf(`printf '{}' > %s/.claude.json`, homeDir),
		}
	case "opencode":
		authCmds = []string{
			fmt.Sprintf("mkdir -p %s/.local/share/opencode", homeDir),
			fmt.Sprintf(`printf '{"token":"fake","provider":"https://api.example.com"}' > %s/.local/share/opencode/auth.json`, homeDir),
		}
	}
	for _, cmd := range authCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "auth setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "auth setup %q exited %d: %s", cmd, code, out)
	}

	// Init a git repo inside the container.
	binPath := filepath.Join(hostDir, "tessariq")
	cmds := []string{
		fmt.Sprintf("mkdir -p %s/tasks", repoPath),
		fmt.Sprintf("git init %s", repoPath),
		fmt.Sprintf("git -C %s config user.email test@test.com", repoPath),
		fmt.Sprintf("git -C %s config user.name Test", repoPath),
		fmt.Sprintf("printf '# Sample Task\\n\\nDo something.\\n' > %s/tasks/sample.md", repoPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m initial", repoPath),
		fmt.Sprintf("cd %s && HOME=%s %s init", repoPath, homeDir, binPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m 'add tessariq config'", repoPath),
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
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")
	imgFlag := fmt.Sprintf("--image tessariq-test-agent-%s-%s", binaryName, testImageTag(t))
	cmd := fmt.Sprintf("cd %s && HOME=%s %s run %s %s tasks/sample.md", repoPath, homeDir, binPath, imgFlag, extraFlags)
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	bin := buildBinary(t)

	ctx := context.Background()
	env, err := containers.StartRunEnvForBinary(ctx, t, "opencode", 0)
	require.NoError(t, err)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	// Copy the tessariq binary.
	binData, readErr := os.ReadFile(bin)
	require.NoError(t, readErr)
	require.NoError(t, os.WriteFile(binPath, binData, 0o755))

	// Setup HOME under bind mount.
	setupCmds := []string{
		fmt.Sprintf("mkdir -p %s", homeDir),
	}
	for _, cmd := range setupCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "setup %q exited %d: %s", cmd, code, out)
	}

	// Create auth.json WITHOUT provider info.
	authCmds := []string{
		fmt.Sprintf("mkdir -p %s/.local/share/opencode", homeDir),
		fmt.Sprintf(`printf '{"token":"fake"}' > %s/.local/share/opencode/auth.json`, homeDir),
	}
	for _, cmd := range authCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "auth setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "auth setup %q exited %d: %s", cmd, code, out)
	}

	// Init git repo.
	cmds := []string{
		fmt.Sprintf("mkdir -p %s/tasks", repoPath),
		fmt.Sprintf("git init %s", repoPath),
		fmt.Sprintf("git -C %s config user.email test@test.com", repoPath),
		fmt.Sprintf("git -C %s config user.name Test", repoPath),
		fmt.Sprintf("printf '# Sample Task\\n\\nDo something.\\n' > %s/tasks/sample.md", repoPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m initial", repoPath),
		fmt.Sprintf("cd %s && HOME=%s %s init", repoPath, homeDir, binPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m 'add tessariq config'", repoPath),
	}
	for _, cmd := range cmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "cmd %q failed: %s", cmd, out)
		require.Equal(t, 0, code, "cmd %q exited %d: %s", cmd, code, out)
	}

	// Run with --agent opencode (default egress=auto) — should fail before container start.
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --agent opencode tasks/sample.md", repoPath, homeDir, binPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "run should fail when provider unresolvable")
	require.Contains(t, output, "configure the provider")
	require.Contains(t, output, "--egress-allow")
}

func TestE2E_OpenCodeEgressAllowBypassesProviderResolution(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	ctx := context.Background()
	env, err := containers.StartRunEnvForBinary(ctx, t, "opencode", 0)
	require.NoError(t, err)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	// Copy the tessariq binary.
	binData, readErr := os.ReadFile(bin)
	require.NoError(t, readErr)
	require.NoError(t, os.WriteFile(binPath, binData, 0o755))

	// Setup HOME under bind mount.
	code, out, execErr := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("mkdir -p %s", homeDir)})
	require.NoError(t, execErr, "home setup: %s", out)
	require.Equal(t, 0, code, "home setup exited %d: %s", code, out)

	// Create auth.json WITHOUT provider info — same as the failure test.
	authCmds := []string{
		fmt.Sprintf("mkdir -p %s/.local/share/opencode", homeDir),
		fmt.Sprintf(`printf '{"token":"fake"}' > %s/.local/share/opencode/auth.json`, homeDir),
	}
	for _, cmd := range authCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "auth setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "auth setup %q exited %d: %s", cmd, code, out)
	}

	// Init git repo.
	cmds := []string{
		fmt.Sprintf("mkdir -p %s/tasks", repoPath),
		fmt.Sprintf("git init %s", repoPath),
		fmt.Sprintf("git -C %s config user.email test@test.com", repoPath),
		fmt.Sprintf("git -C %s config user.name Test", repoPath),
		fmt.Sprintf("printf '# Sample Task\\n\\nDo something.\\n' > %s/tasks/sample.md", repoPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m initial", repoPath),
		fmt.Sprintf("cd %s && HOME=%s %s init", repoPath, homeDir, binPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m 'add tessariq config'", repoPath),
	}
	for _, cmd := range cmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "cmd %q failed: %s", cmd, out)
		require.Equal(t, 0, code, "cmd %q exited %d: %s", cmd, code, out)
	}

	// Run with --agent opencode AND --egress-allow — provider resolution should be skipped.
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --agent opencode --egress-allow api.example.com:443 tasks/sample.md",
			repoPath, homeDir, binPath)})
	require.NoError(t, err)

	// Must NOT contain the provider-unresolvable error.
	require.NotContains(t, output, "configure the provider",
		"--egress-allow should bypass provider resolution; got: %s", output)
	require.NotContains(t, output, "cannot determine the OpenCode provider",
		"--egress-allow should bypass provider resolution; got: %s", output)

	// The run may still fail later (e.g. container image issues in test env),
	// but if it succeeded far enough to write evidence, verify allowlist_source.
	evidenceGlob := fmt.Sprintf("%s/.tessariq/runs/*/manifest.json", repoPath)
	lsCode, lsOut, _ := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("ls %s 2>/dev/null", evidenceGlob)})
	if lsCode == 0 && strings.TrimSpace(lsOut) != "" {
		manifestPath := strings.TrimSpace(strings.Split(lsOut, "\n")[0])
		_, manifest, _ := env.Exec(ctx, []string{"cat", manifestPath})
		require.Contains(t, manifest, `"allowlist_source": "cli"`)
	}
}

func TestE2E_InitFailsWithActionableGuidanceWhenGitMissing(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	ctx := context.Background()
	_, _, err := env.Exec(ctx, []string{"sh", "-c", "mkdir -p /work/bin-empty"})
	require.NoError(t, err)

	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && PATH=/work/bin-empty HOME=%s %s init", repoPath, homeDir, binPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"git\" is missing or unavailable")
	require.Contains(t, output, "install or enable git, then retry")
}

func TestE2E_MountAgentConfigSetsClaudeConfigDir(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	ctx := context.Background()
	// Create a bin dir with only git and docker but not tmux.
	_, _, err := env.Exec(ctx, []string{"sh", "-c",
		"mkdir -p /work/bin && ln -sf $(command -v git) /work/bin/git && ln -sf $(command -v docker) /work/bin/docker"})
	require.NoError(t, err)

	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && PATH=/work/bin HOME=%s %s run tasks/sample.md", repoPath, homeDir, binPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"tmux\" is missing or unavailable")
	require.Contains(t, output, "install or enable tmux, then retry")
	require.NotContains(t, output, "run_id: ")
	require.NotContains(t, output, "attach: tessariq attach ")
	require.NotContains(t, output, "promote: tessariq promote ")
}

func TestE2E_RunFailsWithActionableGuidanceWhenDockerMissing(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	ctx := context.Background()
	// Create a bin dir with git and tmux but not docker.
	_, _, err := env.Exec(ctx, []string{"sh", "-c",
		"mkdir -p /work/bin && ln -sf $(command -v git) /work/bin/git && ln -sf $(command -v tmux) /work/bin/tmux"})
	require.NoError(t, err)

	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && PATH=/work/bin HOME=%s %s run tasks/sample.md", repoPath, homeDir, binPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"docker\" is missing or unavailable")
	require.Contains(t, output, "install or enable docker, then retry")
}

func TestE2E_AgentExecutesInsideContainer(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestE2E_InteractiveOpenCodeRecordsEvidence(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvForBinary(t, bin, "opencode", 0)

	code, output := runTessariq(t, env, "opencode", "--agent opencode --interactive --egress none")
	require.Equal(t, 0, code, "opencode --interactive should succeed: %s", output)
	require.Contains(t, output, "run_id: ")

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	ctx := context.Background()
	agentCode, agentJSON, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "agent.json")})
	require.NoError(t, err)
	require.Equal(t, 0, agentCode, "agent.json must exist")

	var info adapter.AgentInfo
	require.NoError(t, json.Unmarshal([]byte(agentJSON), &info))

	require.Equal(t, "opencode", info.Agent)
	require.Equal(t, true, info.Requested["interactive"],
		"interactive must be recorded as requested")
	require.Equal(t, false, info.Applied["interactive"],
		"opencode must record interactive as not applied")
}

func TestE2E_InteractiveClaudeCodeAccepted(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo interactive-agent-output; exit 0")

	code, output := runTessariq(t, env, "claude", "--interactive")
	require.Equal(t, 0, code, "interactive claude-code should succeed: %s", output)
	require.Contains(t, output, "run_id: ")
}

func TestE2E_ContainerSecurityHardening(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()

	// Verify evidence directory permissions are 700.
	statCode, statOut, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("stat -c '%%a' %s", evidencePath)})
	require.NoError(t, err)
	require.Equal(t, 0, statCode)
	require.Equal(t, "700", strings.TrimSpace(statOut),
		"evidence directory must be 0700")

	// Verify evidence file permissions are 600.
	evidenceFiles := []string{
		"manifest.json", "status.json", "agent.json",
		"runtime.json", "run.log", "runner.log", "task.md",
	}
	for _, f := range evidenceFiles {
		fPath := filepath.Join(evidencePath, f)
		fCode, fOut, fErr := env.Exec(ctx, []string{"sh", "-c",
			fmt.Sprintf("stat -c '%%a' %s", fPath)})
		require.NoError(t, fErr, "%s stat failed", f)
		require.Equal(t, 0, fCode, "%s must exist", f)
		require.Equal(t, "600", strings.TrimSpace(fOut),
			"%s must be 0600", f)
	}
}

func TestE2E_InteractiveClaudeCodeAgentMetadata(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo metadata-test; exit 0")

	code, output := runTessariq(t, env, "claude", "--interactive")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()
	agentCode, agentJSON, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "agent.json")})
	require.NoError(t, err)
	require.Equal(t, 0, agentCode, "agent.json must exist")

	var info adapter.AgentInfo
	require.NoError(t, json.Unmarshal([]byte(agentJSON), &info))

	require.Equal(t, true, info.Requested["interactive"])
	require.Equal(t, true, info.Applied["interactive"])
}

func TestE2E_ProxyModeWritesEgressEvidence(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	// Default egress is "auto" which resolves to "proxy".
	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	ctx := context.Background()

	// Verify egress.compiled.yaml exists and has correct schema.
	catCode, compiledData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "egress.compiled.yaml")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "egress.compiled.yaml must exist: %s", compiledData)
	require.Contains(t, compiledData, "schema_version: 1")
	require.Contains(t, compiledData, "allowlist_source:")
	require.Contains(t, compiledData, "destinations:")

	// Verify egress.events.jsonl exists (may be empty).
	catCode, _, err = env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("test -f %s && echo exists", filepath.Join(evidencePath, "egress.events.jsonl"))})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "egress.events.jsonl must exist")

	// Verify evidence file permissions are 0600.
	for _, f := range []string{"egress.compiled.yaml", "egress.events.jsonl"} {
		fPath := filepath.Join(evidencePath, f)
		fCode, fOut, fErr := env.Exec(ctx, []string{"sh", "-c",
			fmt.Sprintf("stat -c '%%a' %s", fPath)})
		require.NoError(t, fErr, "%s stat failed", f)
		require.Equal(t, 0, fCode, "%s must exist", f)
		require.Equal(t, "600", strings.TrimSpace(fOut), "%s must be 0600", f)
	}
}

func TestE2E_DiffArtifactsWrittenWhenChangesExist(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	// Agent script creates a file in the worktree to produce a diff.
	env := setupRunEnvWithScript(t, bin, "claude", "echo hello > /work/newfile.txt; exit 0")

	code, output := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()

	// diff.patch should exist and contain the new file.
	catCode, patchData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "diff.patch")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "diff.patch must exist: %s", patchData)
	require.Contains(t, patchData, "newfile.txt")

	// diffstat.txt should exist and reference the new file.
	catCode, statData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "diffstat.txt")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "diffstat.txt must exist: %s", statData)
	require.Contains(t, statData, "newfile.txt")

	// Verify 0600 permissions.
	for _, f := range []string{"diff.patch", "diffstat.txt"} {
		fPath := filepath.Join(evidencePath, f)
		fCode, fOut, fErr := env.Exec(ctx, []string{"sh", "-c",
			fmt.Sprintf("stat -c '%%a' %s", fPath)})
		require.NoError(t, fErr, "%s stat failed", f)
		require.Equal(t, 0, fCode, "%s must exist", f)
		require.Equal(t, "600", strings.TrimSpace(fOut), "%s must be 0600", f)
	}
}

func TestE2E_DiffArtifactsAbsentWhenNoChanges(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	// Agent exits immediately with no changes.
	env := setupRunEnv(t, bin, 0)

	code, output := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()

	// diff.patch should NOT exist.
	catCode, _, _ := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("test -f %s", filepath.Join(evidencePath, "diff.patch"))})
	require.NotEqual(t, 0, catCode, "diff.patch must not exist when there are no changes")

	// diffstat.txt should NOT exist.
	catCode, _, _ = env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("test -f %s", filepath.Join(evidencePath, "diffstat.txt"))})
	require.NotEqual(t, 0, catCode, "diffstat.txt must not exist when there are no changes")
}

// setupRunEnvNoBinary creates a RunEnv and builds a test Docker image that does
// NOT contain the agent binary. This is used to test pre-start binary validation.
func setupRunEnvNoBinary(t *testing.T, bin string, agentBinary string) *containers.RunEnv {
	t.Helper()

	ctx := context.Background()
	// Start a RunEnv with a dummy script — we won't use its built-in fake binary.
	env, err := containers.StartRunEnvWithScript(ctx, t, "claude", "exit 0")
	require.NoError(t, err)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")

	// Copy tessariq binary.
	binData, err := os.ReadFile(bin)
	require.NoError(t, err)
	destBin := filepath.Join(hostDir, "tessariq")
	require.NoError(t, os.WriteFile(destBin, binData, 0o755))

	// Setup HOME.
	code, out, execErr := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("mkdir -p %s", homeDir)})
	require.NoError(t, execErr, "home setup: %s", out)
	require.Equal(t, 0, code, "home setup exited %d: %s", code, out)

	// Build a bare Docker image WITHOUT the agent binary.
	imgName := fmt.Sprintf("tessariq-test-no-%s-%s", agentBinary, testImageTag(t))
	buildCmds := []string{
		fmt.Sprintf(`cat > /work/Dockerfile.bare <<'DEOF'
FROM alpine:latest
RUN addgroup -S tessariq && adduser -S tessariq -G tessariq -h /home/tessariq
USER tessariq
WORKDIR /work
DEOF`),
		fmt.Sprintf("docker build -t %s -f /work/Dockerfile.bare /work", imgName),
	}
	for _, cmd := range buildCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "image build %q: %s", cmd, out)
		require.Equal(t, 0, code, "image build %q exited %d: %s", cmd, code, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rmi", "-f", imgName).Run()
	})

	// Create fake auth files.
	var authCmds []string
	switch agentBinary {
	case "claude":
		authCmds = []string{
			fmt.Sprintf("mkdir -p %s/.claude", homeDir),
			fmt.Sprintf(`printf '{"token":"fake"}' > %s/.claude/.credentials.json`, homeDir),
			fmt.Sprintf(`printf '{}' > %s/.claude.json`, homeDir),
		}
	case "opencode":
		authCmds = []string{
			fmt.Sprintf("mkdir -p %s/.local/share/opencode", homeDir),
			fmt.Sprintf(`printf '{"token":"fake","provider":"https://api.example.com"}' > %s/.local/share/opencode/auth.json`, homeDir),
		}
	}
	for _, cmd := range authCmds {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "auth setup %q: %s", cmd, out)
		require.Equal(t, 0, code, "auth setup %q exited %d: %s", cmd, code, out)
	}

	// Init git repo.
	binPath := filepath.Join(hostDir, "tessariq")
	cmds := []string{
		fmt.Sprintf("mkdir -p %s/tasks", repoPath),
		fmt.Sprintf("git init %s", repoPath),
		fmt.Sprintf("git -C %s config user.email test@test.com", repoPath),
		fmt.Sprintf("git -C %s config user.name Test", repoPath),
		fmt.Sprintf("printf '# Sample Task\\n\\nDo something.\\n' > %s/tasks/sample.md", repoPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m initial", repoPath),
		fmt.Sprintf("cd %s && HOME=%s %s init", repoPath, homeDir, binPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m 'add tessariq config'", repoPath),
	}
	for _, cmd := range cmds {
		code, out, err := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, err, "cmd %q failed: %s", cmd, out)
		require.Equal(t, 0, code, "cmd %q exited %d: %s", cmd, code, out)
	}

	return env
}

func TestE2E_MissingAgentBinaryFailsWithGuidance(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvNoBinary(t, bin, "claude")

	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	imgName := fmt.Sprintf("tessariq-test-no-claude-%s", testImageTag(t))
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --image %s --egress none tasks/sample.md",
			repoPath, homeDir, binPath, imgName)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "run should fail when agent binary missing")
	require.Contains(t, output, `"claude"`, "error must name the missing binary")
	require.Contains(t, output, "claude-code", "error must name the selected agent")
	require.Contains(t, output, "--image", "error must suggest --image override")
	require.NotContains(t, output, "run_id:", "must fail before agent start")
}

func TestE2E_MissingOpenCodeBinaryFailsWithGuidance(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvNoBinary(t, bin, "opencode")

	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	imgName := fmt.Sprintf("tessariq-test-no-opencode-%s", testImageTag(t))
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --agent opencode --image %s --egress none tasks/sample.md",
			repoPath, homeDir, binPath, imgName)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "run should fail when agent binary missing")
	require.Contains(t, output, `"opencode"`, "error must name the missing binary")
	require.Contains(t, output, "opencode", "error must name the selected agent")
	require.Contains(t, output, "--image", "error must suggest --image override")
	require.NotContains(t, output, "run_id:", "must fail before agent start")
}

func TestE2E_FailedRunCleansUpWorktree(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	ctx := context.Background()
	// Start a RunEnv container but do NOT create auth files so
	// authmount.Discover fails after worktree provisioning.
	env, err := containers.StartRunEnvWithScript(ctx, t, "claude", "exit 0")
	require.NoError(t, err)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	// Copy the tessariq binary.
	binData, err := os.ReadFile(bin)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(binPath, binData, 0o755))

	// Create HOME directory.
	code, out, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("mkdir -p %s", homeDir)})
	require.NoError(t, err, "mkdir home: %s", out)
	require.Equal(t, 0, code, "mkdir home exited %d: %s", code, out)

	// Initialise a git repo with a task file — no auth files.
	cmds := []string{
		fmt.Sprintf("mkdir -p %s/tasks", repoPath),
		fmt.Sprintf("git init %s", repoPath),
		fmt.Sprintf("git -C %s config user.email test@test.com", repoPath),
		fmt.Sprintf("git -C %s config user.name Test", repoPath),
		fmt.Sprintf("printf '# Sample Task\\n\\nDo something.\\n' > %s/tasks/sample.md", repoPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m initial", repoPath),
		fmt.Sprintf("cd %s && HOME=%s %s init", repoPath, homeDir, binPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m 'add tessariq config'", repoPath),
	}
	for _, cmd := range cmds {
		code, out, err := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, err, "cmd %q: %s", cmd, out)
		require.Equal(t, 0, code, "cmd %q exited %d: %s", cmd, code, out)
	}

	// Run tessariq — should fail at auth discovery (no .claude/.credentials.json).
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --egress none tasks/sample.md",
			repoPath, homeDir, binPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "run should fail when auth files are missing: %s", output)

	// Verify no leaked worktree directories.
	findCode, findOut, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("find %s/.tessariq/worktrees -mindepth 2 -maxdepth 2 -type d 2>/dev/null || true", homeDir)})
	require.NoError(t, err)
	_ = findCode
	require.Empty(t, strings.TrimSpace(findOut),
		"no worktree directories should remain after failed run, found: %s", findOut)

	// Verify git worktree list shows only the main worktree.
	wtCode, wtOut, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("git -C %s worktree list | wc -l", repoPath)})
	require.NoError(t, err)
	require.Equal(t, 0, wtCode)
	require.Equal(t, "1", strings.TrimSpace(wtOut),
		"only the main worktree should exist after failed run")
}

func TestE2E_EvidenceCompletenessAllRequiredFiles(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	code, output := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()

	// All 8 required evidence files must exist and be non-empty.
	requiredFiles := []string{
		"manifest.json", "status.json", "agent.json",
		"runtime.json", "run.log", "runner.log", "task.md",
		"workspace.json",
	}
	for _, f := range requiredFiles {
		fPath := filepath.Join(evidencePath, f)
		catCode, data, err := env.Exec(ctx, []string{"cat", fPath})
		require.NoError(t, err, "%s exec failed", f)
		require.Equal(t, 0, catCode, "%s must exist", f)
		require.NotEmpty(t, data, "%s must be non-empty", f)
	}

	// JSON artifacts must have schema_version: 1.
	jsonFiles := []string{"manifest.json", "status.json", "agent.json", "runtime.json", "workspace.json"}
	for _, f := range jsonFiles {
		fPath := filepath.Join(evidencePath, f)
		_, data, err := env.Exec(ctx, []string{"cat", fPath})
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(data), &raw), "%s must be valid JSON", f)
		require.Contains(t, raw, "schema_version", "%s must have schema_version", f)

		var version int
		require.NoError(t, json.Unmarshal(raw["schema_version"], &version), "%s schema_version must be int", f)
		require.Equal(t, 1, version, "%s schema_version must be 1", f)
	}
}

func TestE2E_CappedRunLogWithOversizedOutput(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	// Agent generates 52 MiB of text output, exceeding the 50 MiB default cap.
	env := setupRunEnvWithScript(t, bin, "claude", `yes | head -c 54525952`)

	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()

	// Check run.log file size.
	statCode, statOut, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("stat -c '%%s' %s", filepath.Join(evidencePath, "run.log"))})
	require.NoError(t, err)
	require.Equal(t, 0, statCode, "run.log must exist")

	var fileSize int64
	_, parseErr := fmt.Sscanf(strings.TrimSpace(statOut), "%d", &fileSize)
	require.NoError(t, parseErr)

	maxAllowed := runner.DefaultLogCapBytes + int64(len(runner.TruncationMarker))
	require.LessOrEqual(t, fileSize, maxAllowed,
		"run.log must not exceed cap + marker (got %d, max %d)", fileSize, maxAllowed)

	// Verify truncation marker at end of file.
	// Use grep to check for the marker; tail -c loses newlines in shell.
	grepCode, _, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("tail -c 20 %s | grep -q '\\[truncated\\]'", filepath.Join(evidencePath, "run.log"))})
	require.NoError(t, err)
	require.Equal(t, 0, grepCode, "run.log must contain truncation marker at end")
}
