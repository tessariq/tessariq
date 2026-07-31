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
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/adapter"
	"github.com/tessariq/tessariq/internal/proxy"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/testutil"
	"github.com/tessariq/tessariq/internal/testutil/containers"
	"gopkg.in/yaml.v3"
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

// testAgentImageFlag renders the --image flag pointing at the per-test agent
// image built by setupRunEnvCustom.
func testAgentImageFlag(t *testing.T, binaryName string) string {
	t.Helper()
	return fmt.Sprintf("--image tessariq-test-agent-%s-%s", binaryName, testImageTag(t))
}

// e2eSetupOpts controls which parts of the standard e2e setup are executed.
// Zero values produce the default behavior (build image, create auth, no extras).
type e2eSetupOpts struct {
	binaryName string                        // agent binary name; defaults to "claude"
	scriptBody string                        // fake agent script body; defaults to "exit 0"
	extraPkgs  string                        // extra apk packages for the agent image (e.g. "curl")
	runtimeUID int                           // runtime image tessariq uid; defaults to 1000
	runtimeGID int                           // runtime image tessariq gid; defaults to runtimeUID
	skipAuth   bool                          // skip creating auth files (for auth-failure tests)
	skipImage  bool                          // skip building test Docker image (for pre-container-failure tests)
	bareImage  bool                          // build image WITHOUT agent binary (for missing-binary tests)
	authFn     func(homeDir string) []string // custom auth commands; receives homeDir, replaces default auth
	extraFn    func(homeDir string) []string // extra commands run after auth, before git init; receives homeDir
}

// setupRunEnvCustom is the single source of truth for e2e test environment setup.
// It creates a RunEnv container, copies the tessariq binary, optionally builds a
// test Docker image, optionally creates auth files, and initialises a git repo.
//
// HOME is set to <hostDir>/home so that worktree, evidence, and auth paths
// are host-absolute and accessible to sibling containers via the Docker socket.
func setupRunEnvCustom(t *testing.T, bin string, opts e2eSetupOpts) *containers.RunEnv {
	t.Helper()

	if opts.binaryName == "" {
		opts.binaryName = "claude"
	}
	if opts.scriptBody == "" {
		opts.scriptBody = "exit 0"
	}
	if opts.runtimeUID == 0 {
		opts.runtimeUID = 1000
	}
	if opts.runtimeGID == 0 {
		opts.runtimeGID = opts.runtimeUID
	}

	ctx := context.Background()
	env, err := containers.StartRunEnvWithScript(ctx, t, opts.binaryName, opts.scriptBody)
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
	execCmd(t, env, ctx, fmt.Sprintf("mkdir -p %s", homeDir), "home setup")

	// Build a test Docker image (unless skipped).
	if !opts.skipImage {
		if opts.bareImage {
			// Bare image WITHOUT the agent binary (for missing-binary tests).
			imgName := fmt.Sprintf("tessariq-test-no-%s-%s", opts.binaryName, testImageTag(t))
			buildCmds := []string{
				fmt.Sprintf(`cat > /work/Dockerfile.bare <<'DEOF'
FROM alpine:latest
RUN apk add --no-cache coreutils \
    && addgroup -g %d tessariq \
    && adduser -D -u %d -G tessariq -h /home/tessariq tessariq
USER tessariq
WORKDIR /work
DEOF`, opts.runtimeGID, opts.runtimeUID),
				fmt.Sprintf("DOCKER_BUILDKIT=1 docker build -t %s -f /work/Dockerfile.bare /work", imgName),
			}
			for _, cmd := range buildCmds {
				execCmd(t, env, ctx, cmd, "bare image build")
			}
			t.Cleanup(func() {
				_ = exec.Command("docker", "rmi", "-f", imgName).Run()
			})
		} else {
			// Standard image with fake agent binary.
			imgName := fmt.Sprintf("tessariq-test-agent-%s-%s", opts.binaryName, testImageTag(t))
			buildCmds := []string{
				fmt.Sprintf(`cat > /work/Dockerfile.test <<'DEOF'
FROM alpine:latest
RUN apk add --no-cache coreutils %s \
    && addgroup -g %d tessariq \
    && adduser -D -u %d -G tessariq -h /home/tessariq tessariq
COPY fake-agent.sh /usr/local/bin/%s
RUN chmod +x /usr/local/bin/%s
USER tessariq
WORKDIR /work
DEOF`, opts.extraPkgs, opts.runtimeGID, opts.runtimeUID, opts.binaryName, opts.binaryName),
				fmt.Sprintf(`printf '#!/bin/sh\n%s\n' > /work/fake-agent.sh && chmod +x /work/fake-agent.sh`,
					strings.ReplaceAll(opts.scriptBody, "'", "'\\''")),
				fmt.Sprintf("DOCKER_BUILDKIT=1 docker build -t %s -f /work/Dockerfile.test /work", imgName),
			}
			for _, cmd := range buildCmds {
				execCmd(t, env, ctx, cmd, "image build")
			}
			t.Cleanup(func() {
				_ = exec.Command("docker", "rmi", "-f", imgName).Run()
			})
		}
	}

	// Create auth files (unless skipped).
	if !opts.skipAuth {
		var authCmds []string
		if opts.authFn != nil {
			authCmds = opts.authFn(homeDir)
		} else {
			switch opts.binaryName {
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
		}
		for _, cmd := range authCmds {
			execCmd(t, env, ctx, cmd, "auth setup")
		}
	}

	// Run any extra setup commands.
	if opts.extraFn != nil {
		for _, cmd := range opts.extraFn(homeDir) {
			execCmd(t, env, ctx, cmd, "extra setup")
		}
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
		execCmd(t, env, ctx, cmd, "git init")
	}

	return env
}

// execCmd runs a shell command inside the RunEnv and fails the test on error.
// It returns the command output for callers that assert on it; setup callers
// invoke it as a statement and ignore the result.
func execCmd(t *testing.T, env *containers.RunEnv, ctx context.Context, cmd, label string) string {
	t.Helper()
	code, out, err := env.Exec(ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err, "%s %q: %s", label, cmd, out)
	require.Equal(t, 0, code, "%s %q exited %d: %s", label, cmd, code, out)
	return out
}

// setupRunEnv creates a RunEnv with the standard setup for the given agent exit code.
func setupRunEnv(t *testing.T, bin string, claudeExitCode int) *containers.RunEnv {
	t.Helper()
	return setupRunEnvForBinary(t, bin, "claude", claudeExitCode)
}

// setupRunEnvForBinary creates a RunEnv with the standard setup for a named agent binary.
func setupRunEnvForBinary(t *testing.T, bin string, binaryName string, exitCode int) *containers.RunEnv {
	t.Helper()
	return setupRunEnvWithScript(t, bin, binaryName, fmt.Sprintf("exit %d", exitCode))
}

// setupRunEnvWithScript creates a RunEnv with a custom agent script body.
func setupRunEnvWithScript(t *testing.T, bin string, binaryName string, scriptBody string) *containers.RunEnv {
	t.Helper()
	return setupRunEnvCustom(t, bin, e2eSetupOpts{
		binaryName: binaryName,
		scriptBody: scriptBody,
	})
}

// runTessariq executes tessariq inside the e2e container with HOME set
// correctly and the test agent image specified via --image. The agent-update
// init container is skipped by default (--no-update-agent) so every e2e test
// does not pay for three extra docker runs (version probe, npm update exec,
// ownership fixup) that duplicate the coverage of the dedicated
// TestE2E_AgentUpdate_* tests. Tests that need to exercise the update path
// should call runTessariqWithUpdate instead.
func runTessariq(t *testing.T, env *containers.RunEnv, binaryName, extraFlags string) (int, string) {
	t.Helper()
	return runTessariqRaw(t, env, binaryName, joinFlags("--no-update-agent", extraFlags))
}

// runTessariqWithUpdate executes tessariq with the agent-update init container
// enabled. Only the dedicated TestE2E_AgentUpdate_* tests should use this —
// every other e2e test should prefer runTessariq.
func runTessariqWithUpdate(t *testing.T, env *containers.RunEnv, binaryName, extraFlags string) (int, string) {
	t.Helper()
	return runTessariqRaw(t, env, binaryName, extraFlags)
}

// runTessariqRaw is the underlying helper that neither adds nor removes
// --no-update-agent. Prefer runTessariq or runTessariqWithUpdate.
func runTessariqRaw(t *testing.T, env *containers.RunEnv, binaryName, flags string) (int, string) {
	t.Helper()
	return runTessariqNoImage(t, env, joinFlags(testAgentImageFlag(t, binaryName), flags))
}

// runTessariqNoImage assembles and runs `tessariq run <flags> tasks/sample.md`
// inside the e2e container with HOME set to the host-absolute home dir. It adds
// no flags of its own, so tests that set skipImage in e2eSetupOpts (the failure
// under test lands before any image is needed) call it directly and pass
// --no-update-agent explicitly.
func runTessariqNoImage(t *testing.T, env *containers.RunEnv, flags string) (int, string) {
	t.Helper()
	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")
	cmd := fmt.Sprintf("cd %s && HOME=%s %s run %s tasks/sample.md", repoPath, homeDir, binPath, flags)
	code, output, err := env.Exec(ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err)
	return code, output
}

// joinFlags concatenates flag fragments with a single space, skipping empty
// fragments so a caller that passes extraFlags="" does not produce a double
// space in the final command line.
func joinFlags(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, " ")
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

func readLastIndexEntry(t *testing.T, env *containers.RunEnv, repoPath string) map[string]any {
	t.Helper()

	ctx := context.Background()
	indexPath := filepath.Join(repoPath, ".tessariq", "runs", "index.jsonl")
	code, data, err := env.Exec(ctx, []string{"cat", indexPath})
	require.NoError(t, err)
	require.Equal(t, 0, code, "index.jsonl must exist")

	lines := strings.Split(strings.TrimSpace(data), "\n")
	require.NotEmpty(t, lines)

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &entry))
	return entry
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

	// Default run (no --interactive) must NOT print the interactive note.
	require.NotContains(t, output, "note: interactive mode without --attach")
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
	require.NotNil(t, agentInfo.Supported)

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
	require.NotNil(t, agentInfo.Supported)
	require.True(t, agentInfo.Supported["interactive"],
		"opencode supports interactive")

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

	// Auth WITHOUT provider info — should cause provider resolution to fail.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		binaryName: "opencode",
		skipImage:  true,
		authFn: func(homeDir string) []string {
			return []string{
				fmt.Sprintf("mkdir -p %s/.local/share/opencode", homeDir),
				fmt.Sprintf(`printf '{"token":"fake"}' > %s/.local/share/opencode/auth.json`, homeDir),
			}
		},
	})

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	// Run with --agent opencode (default egress=auto) — should fail before container start.
	ctx := context.Background()
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

	// Auth WITHOUT provider info — same as the failure test.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		binaryName: "opencode",
		skipImage:  true,
		authFn: func(homeDir string) []string {
			return []string{
				fmt.Sprintf("mkdir -p %s/.local/share/opencode", homeDir),
				fmt.Sprintf(`printf '{"token":"fake"}' > %s/.local/share/opencode/auth.json`, homeDir),
			}
		},
	})

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	// Run with --agent opencode AND --egress-allow — provider resolution should be skipped.
	ctx := context.Background()
	_, output, err := env.Exec(ctx, []string{"sh", "-c",
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

func TestE2E_OpenCodeUserConfigAllowlistBypassesProviderResolution(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// Auth WITHOUT provider info + user config with egress_allow.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		binaryName: "opencode",
		skipImage:  true,
		authFn: func(homeDir string) []string {
			return []string{
				fmt.Sprintf("mkdir -p %s/.local/share/opencode", homeDir),
				fmt.Sprintf(`printf '{"token":"fake"}' > %s/.local/share/opencode/auth.json`, homeDir),
			}
		},
		extraFn: func(homeDir string) []string {
			return []string{
				fmt.Sprintf("mkdir -p %s/.config/tessariq", homeDir),
				fmt.Sprintf(`printf 'egress_allow:\n  - api.example.com:443\n' > %s/.config/tessariq/config.yaml`, homeDir),
			}
		},
	})

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	// Run with --agent opencode and NO --egress-allow — user config should suffice.
	ctx := context.Background()
	_, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --agent opencode tasks/sample.md",
			repoPath, homeDir, binPath)})
	require.NoError(t, err)

	// Must NOT contain the provider-unresolvable error.
	require.NotContains(t, output, "configure the provider",
		"user-config allowlist should bypass provider resolution; got: %s", output)
	require.NotContains(t, output, "cannot determine the OpenCode provider",
		"user-config allowlist should bypass provider resolution; got: %s", output)

	// If evidence was written, verify allowlist_source is user_config.
	evidenceGlob := fmt.Sprintf("%s/.tessariq/runs/*/manifest.json", repoPath)
	lsCode, lsOut, _ := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("ls %s 2>/dev/null", evidenceGlob)})
	if lsCode == 0 && strings.TrimSpace(lsOut) != "" {
		manifestPath := strings.TrimSpace(strings.Split(lsOut, "\n")[0])
		_, manifest, _ := env.Exec(ctx, []string{"cat", manifestPath})
		require.Contains(t, manifest, `"allowlist_source": "user_config"`)
	}
}

func TestE2E_ExplicitEgressOpenIgnoresMalformedUserConfig(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		extraFn: func(homeDir string) []string {
			return []string{
				fmt.Sprintf("mkdir -p %s/.config/tessariq", homeDir),
				fmt.Sprintf(`printf '{{invalid yaml' > %s/.config/tessariq/config.yaml`, homeDir),
			}
		},
	})

	code, output := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, code, "run failed: %s", output)

	require.NotContains(t, output, "malformed config file",
		"--egress open should skip user config; got: %s", output)
}

func TestE2E_ExplicitEgressAllowIgnoresMalformedUserConfig(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		extraFn: func(homeDir string) []string {
			return []string{
				fmt.Sprintf("mkdir -p %s/.config/tessariq", homeDir),
				fmt.Sprintf(`printf '{{invalid yaml' > %s/.config/tessariq/config.yaml`, homeDir),
			}
		},
	})

	code, output := runTessariq(t, env, "claude", "--egress-allow api.example.com:443")
	require.Equal(t, 0, code, "run failed: %s", output)

	require.NotContains(t, output, "malformed config file",
		"--egress-allow should skip user config; got: %s", output)
}

func TestE2E_UnknownFieldUserConfigFailsLoudly(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		extraFn: func(homeDir string) []string {
			return []string{
				fmt.Sprintf("mkdir -p %s/.config/tessariq", homeDir),
				fmt.Sprintf(`printf 'egressAllow:\n  - api.example.com:443\n' > %s/.config/tessariq/config.yaml`, homeDir),
			}
		},
	})

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run tasks/sample.md",
			repoPath, homeDir, binPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "expected non-zero exit for unknown config field")
	require.Contains(t, output, "unknown field",
		"typoed user config should fail with unknown field error; got: %s", output)
	require.Contains(t, output, "config.yaml",
		"error should identify the config file path; got: %s", output)
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

func TestE2E_DetachedRunsAppendTerminalIndexEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		binaryName string
		extraFlags string
		agent      string
	}{
		{name: "claude-code", binaryName: "claude", agent: "claude-code"},
		{name: "opencode", binaryName: "opencode", extraFlags: "--agent opencode", agent: "opencode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bin := buildBinary(t)
			env := setupRunEnvForBinary(t, bin, tt.binaryName, 0)
			hostDir := env.Dir()
			repoPath := filepath.Join(hostDir, "repo")

			code, output := runTessariq(t, env, tt.binaryName, tt.extraFlags)
			require.Equal(t, 0, code, "run failed: %s", output)

			runID := extractField(output, "run_id")
			evidencePath := extractField(output, "evidence_path")
			require.NotEmpty(t, runID)
			require.NotEmpty(t, evidencePath)

			ctx := context.Background()
			statusCode, statusData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "status.json")})
			require.NoError(t, err)
			require.Equal(t, 0, statusCode, "status.json must exist")

			var status map[string]any
			require.NoError(t, json.Unmarshal([]byte(statusData), &status))
			require.Equal(t, "success", status["state"])

			indexEntry := readLastIndexEntry(t, env, repoPath)
			require.Equal(t, runID, indexEntry["run_id"])
			require.Equal(t, "success", indexEntry["state"])
			require.Equal(t, tt.agent, indexEntry["agent"])
		})
	}
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
	testutil.WaitFor(t, 5*time.Second, func() bool {
		code, _, _ := env.Exec(ctx, []string{"sh", "-c", "tmux has-session -t " + sessionName})
		return code == 0
	}, "tmux session %s should exist after run completes", sessionName)

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
	require.Equal(t, true, info.Supported["interactive"],
		"opencode must record interactive as supported")
}

func TestE2E_InteractiveClaudeCodeAccepted(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo interactive-agent-output; exit 0")

	code, output := runTessariq(t, env, "claude", "--interactive")
	require.Equal(t, 0, code, "interactive claude-code should succeed: %s", output)
	require.Contains(t, output, "run_id: ")

	// Explicit --interactive without --attach must print the note.
	require.Contains(t, output, "note: interactive mode without --attach")
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

	// Verify worktree perms — T-093 hardening. The worktree parent chain
	// and the worktree root itself must be owner-only at the mode-bit layer,
	// with the runtime uid granted exact access via ACL instead of host gid
	// membership.
	wsPath := extractField(output, "workspace_path")
	require.NotEmpty(t, wsPath, "workspace_path must be printed on success")

	aclCode, aclOut, aclErr := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("getfacl -cp %s", wsPath)})
	require.NoError(t, aclErr)
	require.Equal(t, 0, aclCode)
	require.Contains(t, aclOut, "other::---",
		"worktree ACL must deny all other users")
	require.Contains(t, aclOut, "user:1000:rwx",
		"runtime uid must be granted explicit ACL access")

	// Parent chain must be owner-only so non-owner users cannot list run IDs.
	worktreesParent := filepath.Dir(filepath.Dir(wsPath))
	for _, parent := range []string{filepath.Dir(wsPath), worktreesParent} {
		pCode, pOut, pErr := env.Exec(ctx, []string{"sh", "-c",
			fmt.Sprintf("stat -c '%%a' %s", parent)})
		require.NoError(t, pErr)
		require.Equal(t, 0, pCode, "%s must exist", parent)
		require.Equal(t, "700", strings.TrimSpace(pOut),
			"%s must be 0700", parent)
	}
}

func TestE2E_CustomRuntimeUIDCanUseHardenedWorktree(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		runtimeUID: 1234,
		runtimeGID: 1234,
	})

	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	ctx := context.Background()
	wsPath := extractField(output, "workspace_path")
	require.NotEmpty(t, wsPath)

	aclCode, aclOut, aclErr := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("getfacl -cp %s", wsPath)})
	require.NoError(t, aclErr)
	require.Equal(t, 0, aclCode)
	require.Contains(t, aclOut, "user:1234:rwx")

	thirdPartyCode, thirdPartyOut, thirdPartyErr := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("docker run --rm --user 4242:4242 -v %s:/work alpine:latest sh -c 'cat /work/.git'", wsPath)})
	require.NoError(t, thirdPartyErr)
	require.NotEqual(t, 0, thirdPartyCode)
	require.Contains(t, strings.ToLower(thirdPartyOut), "permission denied")
}

// TestE2E_ClaudeJsonHostFileImmutableAfterRun is the BUG-050 regression guard
// at the e2e level. The fake claude agent mutates ~/.claude.json inside the
// container before exiting. The test asserts that the host-side .claude.json
// file (under the e2e home dir) is byte-identical to its pre-run content,
// proving that the disposable per-run runtime-state layer prevents
// container-to-host persistence.
func TestE2E_ClaudeJsonHostFileImmutableAfterRun(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	// Fake agent: overwrites its own ~/.claude.json with a mutation marker.
	// If the host bind were writable (BUG-050), the host file under
	// <hostDir>/home/.claude.json would pick up this mutation.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		binaryName: "claude",
		scriptBody: `echo '{"mutated_from_container":true,"numStartups":9999}' > "$HOME/.claude.json"`,
	})

	hostDir := env.Dir()
	homeDir := filepath.Join(hostDir, "home")
	hostClaudeJSON := filepath.Join(homeDir, ".claude.json")

	ctx := context.Background()

	// Capture original host content.
	origCode, origContent, err := env.Exec(ctx, []string{"cat", hostClaudeJSON})
	require.NoError(t, err)
	require.Equal(t, 0, origCode, "host .claude.json must exist before run")

	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	// Host file must be byte-identical to its pre-run content.
	postCode, postContent, err := env.Exec(ctx, []string{"cat", hostClaudeJSON})
	require.NoError(t, err)
	require.Equal(t, 0, postCode, "host .claude.json must still exist after run")
	require.Equal(t, origContent, postContent,
		"host .claude.json must not be mutated by in-container writes (BUG-050 regression)")
	require.NotContains(t, postContent, "mutated_from_container",
		"in-container mutation must not leak to host auth file")

	// The scratch file for this run must be removed after run cleanup.
	runID := extractField(output, "run_id")
	require.NotEmpty(t, runID)
	scratchDir := filepath.Join(homeDir, ".tessariq", "runtime-state", runID)
	checkCode, _, err := env.Exec(ctx, []string{"test", "-e", scratchDir})
	require.NoError(t, err)
	require.NotEqual(t, 0, checkCode,
		"runtime-state scratch directory %s must be removed after the run completes", scratchDir)
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
	require.Equal(t, true, info.Supported["interactive"])
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

	// Verify egress.events.jsonl exists and is non-empty (summary line for zero events).
	catCode, _, err = env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("test -s %s && echo exists", filepath.Join(evidencePath, "egress.events.jsonl"))})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "egress.events.jsonl must exist and be non-empty")

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

func TestE2E_ProxyModeMultipleDestinations(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	// Run with multiple destinations on different ports.
	code, output := runTessariq(t, env, "claude", "--egress-allow httpbin.org:443 --egress-allow example.com:8443")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	ctx := context.Background()

	// Verify egress.compiled.yaml contains both destinations.
	catCode, compiledData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "egress.compiled.yaml")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "egress.compiled.yaml must exist: %s", compiledData)
	require.Contains(t, compiledData, "host: httpbin.org")
	require.Contains(t, compiledData, "port: 443")
	require.Contains(t, compiledData, "host: example.com")
	require.Contains(t, compiledData, "port: 8443")
}

// TestE2E_BlockedDestinationIsReportedAndCanBeAllowed covers manual case T3.3:
// when the agent reaches for a host outside the compiled allowlist, Squid must
// deny it, the denial must be recorded in egress.events.jsonl as a blocked
// event, and the CLI must name the flag that allows it. Re-running with the
// destination on --egress-allow must clear both the denial and the guidance.
func TestE2E_BlockedDestinationIsReportedAndCanBeAllowed(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	const deniedHost = "denied.example.com"
	const deniedHostPort = deniedHost + ":443"

	// The fake agent probes a host that is not in the built-in profile. Squid
	// denies the CONNECT, so curl reports the proxy's 403 and no real network
	// egress happens.
	script := fmt.Sprintf("curl -sS --max-time 15 https://%s/ -o /dev/null 2>/work/probe.txt; "+
		"echo curl_exit=$? >> /work/probe.txt; exit 0", deniedHost)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{scriptBody: script, extraPkgs: "curl"})

	// Default egress is "auto", which resolves to proxy.
	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run should succeed (agent exits 0): %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")
	wsPath := extractField(output, "workspace_path")
	require.NotEmpty(t, wsPath, "workspace_path must be in output")

	// The proxy refused the CONNECT rather than letting curl reach the network.
	probe := readFileInEnv(t, env, filepath.Join(wsPath, "probe.txt"))
	require.NotContains(t, probe, "curl_exit=0",
		"%s must be unreachable while it is outside the allowlist: %s", deniedHostPort, probe)
	require.Contains(t, probe, "403",
		"the proxy must refuse the CONNECT with 403: %s", probe)

	// The denial is recorded with its host:port and a reason.
	blocked := readBlockedEvents(t, env, evidencePath)
	event, ok := blocked[deniedHostPort]
	require.True(t, ok, "a blocked event for %s must be recorded, got %v", deniedHostPort, blocked)
	require.Equal(t, "blocked", event.Action)
	require.Equal(t, "not_in_allowlist", event.Reason)
	require.Contains(t, event.SquidResult, "TCP_DENIED",
		"the event must carry the Squid result that proves the denial")
	require.NotEmpty(t, event.Timestamp, "the event must be timestamped")

	// The CLI tells the operator which destination was blocked and how to allow it.
	require.Contains(t, output, "Blocked egress destinations:")
	require.Contains(t, output, deniedHostPort,
		"guidance must name the blocked destination: %s", output)
	require.Contains(t, output, "--egress-allow",
		"guidance must name the flag that allows the destination: %s", output)

	// Allowing the destination clears both the denial and the guidance.
	code, output = runTessariq(t, env, "claude", "--egress-allow "+deniedHostPort)
	require.Equal(t, 0, code, "run should succeed (agent exits 0): %s", output)

	evidencePath = extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")
	wsPath = extractField(output, "workspace_path")
	require.NotEmpty(t, wsPath, "workspace_path must be in output")

	require.Contains(t, compiledHostPorts(readCompiledAllowlist(t, env, evidencePath)), deniedHostPort,
		"--egress-allow must add the destination to the compiled allowlist")

	probe = readFileInEnv(t, env, filepath.Join(wsPath, "probe.txt"))
	require.NotContains(t, probe, "403",
		"the proxy must no longer refuse the CONNECT once the destination is allowed: %s", probe)

	blocked = readBlockedEvents(t, env, evidencePath)
	require.NotContains(t, blocked, deniedHostPort,
		"an allowed destination must not be recorded as blocked, got %v", blocked)
	require.NotContains(t, output, "Blocked egress destinations:",
		"no guidance may be printed when nothing was blocked: %s", output)
}

// TestE2E_ProxyZeroDeniedRunEmitsSummaryAndPromotes covers manual case T3.4 and
// guards the BUG-064 regression: a proxy run that had nothing denied must still
// write an explicit zero-events summary to egress.events.jsonl. A 0-byte file is
// indistinguishable from a truncated telemetry write, so the promote
// completeness gate rejects it and an honest run becomes unpromotable.
func TestE2E_ProxyZeroDeniedRunEmitsSummaryAndPromotes(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// The agent makes no network calls at all, so Squid denies nothing. It does
	// change a file so the run has a diff to promote.
	env := setupRunEnvWithScript(t, bin, "claude", "echo promoted > /work/promoted.txt; exit 0")

	// Default egress is "auto", which resolves to proxy.
	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	runID := extractField(output, "run_id")
	require.NotEmpty(t, runID, "run_id must be in output")
	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	var manifest run.Manifest
	manifestData := readFileInEnv(t, env, filepath.Join(evidencePath, "manifest.json"))
	require.NoError(t, json.Unmarshal([]byte(manifestData), &manifest), "manifest.json must parse: %s", manifestData)
	require.Equal(t, "proxy", manifest.ResolvedEgressMode,
		"the zero-denied case is only meaningful when the proxy was actually in front of the agent")

	// egress.events.jsonl is an explicit zero-events summary, not a 0-byte file.
	raw := readFileInEnv(t, env, filepath.Join(evidencePath, "egress.events.jsonl"))
	lines := nonEmptyLines(raw)
	require.Len(t, lines, 1, "a zero-denied run must write exactly one summary line: %q", raw)

	var summary proxy.EventsSummary
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &summary), "summary line must parse: %s", lines[0])
	require.Equal(t, 1, summary.SchemaVersion, "summary must carry the events schema version: %s", lines[0])
	require.Equal(t, 0, summary.EventCount, "summary must state that zero destinations were denied: %s", lines[0])

	require.Empty(t, readBlockedEvents(t, env, evidencePath), "no destination was denied")
	require.NotContains(t, output, "Blocked egress destinations:",
		"no guidance may be printed when nothing was blocked: %s", output)

	// The honest zero-denied run is promotable.
	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.Equal(t, 0, promoteCode, "a zero-denied proxy run must be promotable: %s", promoteOutput)
	require.Contains(t, promoteOutput, "branch: tessariq/"+runID)
}

// nonEmptyLines splits raw evidence text into its non-blank lines.
func nonEmptyLines(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// readBlockedEvents parses egress.events.jsonl from an evidence directory inside
// the e2e container and indexes the blocked events by "host:port" so tests can
// assert on presence and absence of a specific destination. The zero-events
// summary line carries no host, so it is skipped rather than decoded as a
// blocked event.
func readBlockedEvents(t *testing.T, env *containers.RunEnv, evidencePath string) map[string]proxy.Event {
	t.Helper()
	raw := readFileInEnv(t, env, filepath.Join(evidencePath, "egress.events.jsonl"))

	blocked := make(map[string]proxy.Event)
	for _, line := range nonEmptyLines(raw) {
		var event proxy.Event
		require.NoError(t, json.Unmarshal([]byte(line), &event), "event line must parse: %s", line)
		if event.Host == "" {
			continue
		}
		blocked[fmt.Sprintf("%s:%d", event.Host, event.Port)] = event
	}
	return blocked
}

// readFileInEnv reads a file from inside the e2e container and fails the test
// when it is missing or unreadable.
func readFileInEnv(t *testing.T, env *containers.RunEnv, path string) string {
	t.Helper()
	code, data, err := env.Exec(context.Background(), []string{"cat", path})
	require.NoError(t, err)
	require.Equal(t, 0, code, "%s must exist: %s", path, data)
	return data
}

// readCompiledAllowlist reads and parses egress.compiled.yaml from an evidence
// directory inside the e2e container.
func readCompiledAllowlist(t *testing.T, env *containers.RunEnv, evidencePath string) *proxy.CompiledAllowlist {
	t.Helper()
	data := readFileInEnv(t, env, filepath.Join(evidencePath, "egress.compiled.yaml"))

	var compiled proxy.CompiledAllowlist
	require.NoError(t, yaml.Unmarshal([]byte(data), &compiled), "egress.compiled.yaml must parse: %s", data)
	require.NoError(t, compiled.Validate(), "egress.compiled.yaml must satisfy the spec contract: %s", data)
	return &compiled
}

// compiledHostPorts renders a compiled allowlist as a sorted "host:port" slice
// so tests can assert exact set equality rather than mere containment. Exact
// equality is what makes a silently widened allowlist a test failure.
func compiledHostPorts(c *proxy.CompiledAllowlist) []string {
	out := make([]string, 0, len(c.Destinations))
	for _, d := range c.Destinations {
		out = append(out, fmt.Sprintf("%s:%d", d.Host, d.Port))
	}
	sort.Strings(out)
	return out
}

// builtInClaudeCodeHostPorts is the full built-in profile for Claude Code:
// the shared baseline endpoints plus the Claude Code agent endpoints.
//
// The expectation is derived from the adapter rather than hardcoded on purpose:
// this test guards the compile path (profile -> egress.compiled.yaml), while the
// contents of the profile itself are pinned exactly at unit level by
// TestBaselineEndpoints_Count/_RequiredHosts and
// TestClaudeCodeEndpoints_Count/_RequiredHosts, where count plus
// all-required-present together forbid both added and substituted hosts.
// Restating the list here would duplicate that guard, not strengthen it.
func builtInClaudeCodeHostPorts() []string {
	dests := adapter.FullBuiltInAllowlist(adapter.ClaudeCodeEndpoints())
	out := make([]string, 0, len(dests))
	for _, d := range dests {
		out = append(out, d.String())
	}
	sort.Strings(out)
	return out
}

// readManifestAllowlistSource reads allowlist_source from manifest.json, so
// tests can check that the manifest and egress.compiled.yaml agree.
func readManifestAllowlistSource(t *testing.T, env *containers.RunEnv, evidencePath string) string {
	t.Helper()
	data := readFileInEnv(t, env, filepath.Join(evidencePath, "manifest.json"))

	var manifest run.Manifest
	require.NoError(t, json.Unmarshal([]byte(data), &manifest), "manifest.json must parse: %s", data)
	require.NotEmpty(t, manifest.AllowlistSource, "manifest.json must record allowlist_source: %s", data)
	return manifest.AllowlistSource
}

// TestE2E_EgressAutoCompilesBuiltInProfile covers manual case T3.1: under the
// default --egress auto, the compiled allowlist must be exactly the built-in
// Claude Code profile and its provenance must be recorded as built_in.
func TestE2E_EgressAutoCompilesBuiltInProfile(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	code, output := runTessariq(t, env, "claude", "--egress auto")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	compiled := readCompiledAllowlist(t, env, evidencePath)
	require.Equal(t, "built_in", compiled.AllowlistSource,
		"auto egress with no CLI or user-config entries must record built_in provenance")
	require.Equal(t, builtInClaudeCodeHostPorts(), compiledHostPorts(compiled),
		"compiled allowlist must be exactly the built-in Claude Code profile")

	require.Equal(t, "built_in", readManifestAllowlistSource(t, env, evidencePath),
		"manifest.json must agree with egress.compiled.yaml on provenance")
}

// TestE2E_EgressNoDefaultsDropsBuiltInProfile covers manual case T3.7:
// --egress-no-defaults with explicit --egress-allow entries must compile only
// those entries, and the agent must be unable to reach its own API — the
// reachability probe is what proves the flag took effect rather than merely
// being recorded in evidence.
func TestE2E_EgressNoDefaultsDropsBuiltInProfile(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// The fake agent probes its own API host through the injected proxy. Squid
	// denies the CONNECT because api.anthropic.com is no longer allowlisted, so
	// curl exits non-zero without any real network egress.
	script := "curl -sS --max-time 15 https://api.anthropic.com/ -o /dev/null 2>/work/probe.txt; " +
		"echo curl_exit=$? >> /work/probe.txt; exit 0"
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		scriptBody: script,
		extraPkgs:  "curl",
	})

	code, output := runTessariq(t, env, "claude",
		"--egress-no-defaults --egress-allow allowed.example.com:443")
	require.Equal(t, 0, code, "run should succeed (agent exits 0): %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	compiled := readCompiledAllowlist(t, env, evidencePath)
	require.Equal(t, "cli", compiled.AllowlistSource,
		"CLI entries must be recorded as the allowlist source")
	require.Equal(t, []string{"allowed.example.com:443"}, compiledHostPorts(compiled),
		"--egress-no-defaults must discard the built-in profile entirely")

	// The agent could not reach api.anthropic.com.
	wsPath := extractField(output, "workspace_path")
	require.NotEmpty(t, wsPath, "workspace_path must be in output")
	probe := readFileInEnv(t, env, filepath.Join(wsPath, "probe.txt"))
	require.NotContains(t, probe, "curl_exit=0",
		"api.anthropic.com must be unreachable once the built-in profile is dropped: %s", probe)

	// The denial is recorded as a blocked egress event.
	blocked := readBlockedEvents(t, env, evidencePath)
	event, ok := blocked["api.anthropic.com:443"]
	require.True(t, ok,
		"a blocked event for api.anthropic.com:443 must be recorded, got: %v", blocked)
	require.Equal(t, "blocked", event.Action)
	require.Equal(t, "not_in_allowlist", event.Reason)
}

// TestE2E_EgressNoDefaultsWithoutEntriesRejected covers the empty-allowlist
// corner of manual case T3.7: proxy mode with no destination at all leaves the
// agent nothing to reach, so the run is refused before any evidence is written.
func TestE2E_EgressNoDefaultsWithoutEntriesRejected(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{skipImage: true})

	code, output := runTessariqNoImage(t, env, "--egress-no-defaults --no-update-agent")
	require.NotEqual(t, 0, code,
		"proxy mode with an empty allowlist must be refused: %s", output)
	require.Contains(t, output, "proxy mode requires at least one allowlist destination",
		"error must explain that proxy mode has no reachable destination: %s", output)
	require.Contains(t, output, "--egress-allow",
		"error must point at --egress-allow: %s", output)

	require.NotContains(t, output, "run_id:", "must fail before evidence bootstrap: %s", output)
	requireNoRunDirectory(t, env)
	requireNoContainerForEnv(t, env)
}

// TestE2E_EgressOpenWithAllowRejected covers manual case T3.6: an allowlist is
// meaningless without the proxy, so combining it with --egress open must fail
// validation and steer the operator toward proxy mode.
func TestE2E_EgressOpenWithAllowRejected(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{skipImage: true})

	code, output := runTessariqNoImage(t, env,
		"--egress open --egress-allow api.example.com:443 --no-update-agent")
	require.NotEqual(t, 0, code,
		"--egress open with --egress-allow must be rejected: %s", output)
	require.Contains(t, output, "allowlists require proxy mode",
		"error must steer toward proxy mode: %s", output)

	require.NotContains(t, output, "run_id:", "must fail before evidence bootstrap: %s", output)
	requireNoRunDirectory(t, env)
	requireNoContainerForEnv(t, env)
}

// TestE2E_CLIEgressAllowOverridesUserConfig asserts the CLI > user_config leg of
// allowlist precedence: user-config entries must not leak into the compiled
// allowlist when --egress-allow is present, and provenance must read cli.
func TestE2E_CLIEgressAllowOverridesUserConfig(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		extraFn: func(homeDir string) []string {
			return []string{
				fmt.Sprintf("mkdir -p %s/.config/tessariq", homeDir),
				fmt.Sprintf(`printf 'egress_allow:\n  - from-config.example.com:443\n' > %s/.config/tessariq/config.yaml`, homeDir),
			}
		},
	})

	code, output := runTessariq(t, env, "claude", "--egress-allow from-cli.example.com:443")
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	compiled := readCompiledAllowlist(t, env, evidencePath)
	require.Equal(t, "cli", compiled.AllowlistSource,
		"CLI entries must be recorded as the allowlist source")
	require.Equal(t, []string{"from-cli.example.com:443"}, compiledHostPorts(compiled),
		"CLI entries must fully replace user-config entries")

	require.Equal(t, "cli", readManifestAllowlistSource(t, env, evidencePath),
		"manifest.json must agree with egress.compiled.yaml on provenance")
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

// setupRunEnvNoBinary creates a RunEnv with a bare Docker image that does NOT
// contain the agent binary. Used to test pre-start binary validation.
func setupRunEnvNoBinary(t *testing.T, bin string, agentBinary string) *containers.RunEnv {
	t.Helper()
	return setupRunEnvCustom(t, bin, e2eSetupOpts{
		binaryName: agentBinary,
		bareImage:  true,
	})
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
	// Post-bootstrap failure must surface evidence locators.
	require.Contains(t, output, "run_id:", "post-bootstrap failure must print run_id")
	require.Contains(t, output, "evidence_path:", "post-bootstrap failure must print evidence_path")
	require.NotContains(t, output, "workspace_path:", "success-only field must not appear on failure")
	require.NotContains(t, output, "attach:", "success-only field must not appear on failure")
	require.NotContains(t, output, "promote:", "success-only field must not appear on failure")
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

	// Post-bootstrap failure must surface evidence locators.
	require.Contains(t, output, "run_id:", "post-bootstrap failure must print run_id")
	require.Contains(t, output, "evidence_path:", "post-bootstrap failure must print evidence_path")
	require.NotContains(t, output, "workspace_path:", "success-only field must not appear on failure")
	require.NotContains(t, output, "attach:", "success-only field must not appear on failure")
	require.NotContains(t, output, "promote:", "success-only field must not appear on failure")
}

// requireRunResourcesCleanedUp is the post-run resource hygiene sweep for the
// resources a run must always release, whatever its outcome: the agent
// container, the Squid sidecar, the per-run Docker network, and the disposable
// runtime-state layer that holds a copy of the host auth file.
//
// The worktree is deliberately excluded: a successful run keeps it as the
// promotable workspace, so its lifecycle is asserted separately by
// requireWorktreeRemoved and requireWorktreeRetained.
//
// Every check filters on the deterministic per-run name rather than counting
// resources, so an unrelated container or network on the runner can neither
// mask a leak nor cause a false failure.
func requireRunResourcesCleanedUp(t *testing.T, env *containers.RunEnv, runID string) {
	t.Helper()
	require.NotEmpty(t, runID, "hygiene sweep needs the run id")

	ctx := context.Background()

	for _, name := range []string{"tessariq-" + runID, "tessariq-squid-" + runID} {
		out := execCmd(t, env, ctx,
			fmt.Sprintf("docker ps -a --format '{{.Names}}' --filter name=^%s$", name), "docker ps")
		require.Empty(t, strings.TrimSpace(out), "container %s must not survive the run", name)
	}

	netName := "tessariq-net-" + runID
	netOut := execCmd(t, env, ctx,
		fmt.Sprintf("docker network ls --format '{{.Name}}' --filter name=^%s$", netName), "docker network ls")
	require.Empty(t, strings.TrimSpace(netOut), "network %s must not survive the run", netName)

	// Catch-all for any container this environment produced under a name the
	// deterministic checks above do not anticipate.
	requireNoContainerForEnv(t, env)

	// The disposable runtime-state layer holds a copy of host credentials, so
	// its removal is a security guarantee rather than mere tidiness.
	stateRoot := filepath.Join(env.Dir(), "home", ".tessariq", "runtime-state")
	stateDir := filepath.Join(stateRoot, runID)
	stateCode, stateOut, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("test -e %s", stateDir)})
	require.NoError(t, err)
	require.NotEqual(t, 0, stateCode,
		"runtime-state directory %s must be removed after the run: %s", stateDir, stateOut)

	// Nothing may survive anywhere under the runtime-state root either, which
	// catches a copied auth file stranded outside this run's directory.
	leftoverOut := execCmd(t, env, ctx,
		fmt.Sprintf("find %s -type f 2>/dev/null || true", stateRoot), "find runtime-state files")
	require.Empty(t, strings.TrimSpace(leftoverOut),
		"no copied host auth or state file may remain under runtime-state, found: %s", leftoverOut)
}

// requireWorktreeRemoved asserts a non-success run left no worktree behind,
// neither on disk nor in git's own bookkeeping.
func requireWorktreeRemoved(t *testing.T, env *containers.RunEnv, runID string) {
	t.Helper()

	worktreeRoot := filepath.Join(env.Dir(), "home", ".tessariq", "worktrees")
	findOut := execCmd(t, env, context.Background(),
		fmt.Sprintf("find %s -mindepth 2 -maxdepth 2 -type d 2>/dev/null || true", worktreeRoot),
		"find worktree dirs")
	require.Empty(t, strings.TrimSpace(findOut),
		"no worktree directory should remain, found: %s", findOut)

	wtOut := gitWorktreeList(t, env)
	require.NotContains(t, wtOut, runID, "git worktree list must have no stale entry for %s", runID)
	require.Len(t, nonEmptyLines(wtOut), 1, "only the main worktree should remain: %s", wtOut)
}

// requireWorktreeRetained asserts the opposite for a successful run: the
// workspace the CLI advertised is still there, both on disk and in git's
// bookkeeping, because `tessariq promote` commits from it. Removing it would
// break the run -> attach -> promote flow, so this pins the retention as
// intended behavior rather than a leak the hygiene sweep missed.
func requireWorktreeRetained(t *testing.T, env *containers.RunEnv, runID, workspacePath string) {
	t.Helper()
	require.NotEmpty(t, workspacePath, "successful run must advertise workspace_path")
	require.Contains(t, workspacePath, runID, "workspace path must be per-run")

	execCmd(t, env, context.Background(), fmt.Sprintf("test -d %s", workspacePath),
		"successful run must retain its promotable workspace")

	wtOut := gitWorktreeList(t, env)
	require.Contains(t, wtOut, runID, "git worktree list must still track the promotable workspace: %s", wtOut)
	require.Len(t, nonEmptyLines(wtOut), 2, "main worktree plus the run workspace: %s", wtOut)
}

// gitWorktreeList returns `git worktree list` output for the e2e repo.
func gitWorktreeList(t *testing.T, env *containers.RunEnv) string {
	t.Helper()
	return execCmd(t, env, context.Background(),
		fmt.Sprintf("git -C %s worktree list", filepath.Join(env.Dir(), "repo")),
		"git worktree list")
}

// TestE2E_SuccessfulRunLeavesNoResources is the success-path counterpart to
// TestE2E_FailedRunCleansUpWorktree. A leak on the happy path is the worst
// kind: nothing looks wrong to the user while containers, networks, and copies
// of host credentials accumulate run after run. The worktree is the one
// resource a successful run keeps on purpose — it is the workspace `tessariq
// promote` commits from.
func TestE2E_SuccessfulRunLeavesNoResources(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	// Default egress ("auto") resolves to proxy, so this run really does
	// create a Squid container and a per-run network for the sweep to check.
	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run failed: %s", output)

	runID := extractField(output, "run_id")
	requireRunResourcesCleanedUp(t, env, runID)
	requireWorktreeRetained(t, env, runID, extractField(output, "workspace_path"))
}

// TestE2E_TimedOutRunLeavesNoResources proves cleanup on more than the happy
// path: a run killed by its own timeout budget releases everything, worktree
// included, since there is no promotable result to keep.
func TestE2E_TimedOutRunLeavesNoResources(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "sleep 300")

	code, output := runTessariq(t, env, "claude", "--timeout 3s --grace 1s")
	require.NotEqual(t, 0, code, "timed-out run must exit non-zero: %s", output)
	require.Contains(t, output, "state: timeout")

	runID := extractField(output, "run_id")
	requireRunResourcesCleanedUp(t, env, runID)
	requireWorktreeRemoved(t, env, runID)
}

func TestE2E_FailedRunCleansUpWorktree(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// No auth files → authmount.Discover fails after worktree provisioning.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		skipAuth:  true,
		skipImage: true,
	})

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	// Run tessariq — should fail at auth discovery (no .claude/.credentials.json).
	ctx := context.Background()
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

func TestE2E_PostBootstrapFailurePrintsEvidencePath(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// No auth files → authmount.Discover fails after evidence bootstrap.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		skipAuth:  true,
		skipImage: true,
	})

	ctx := context.Background()
	code, output := runTessariqNoImage(t, env, "--egress none")
	require.NotEqual(t, 0, code, "run should fail when auth files are missing")

	// Post-bootstrap failure must print run_id and evidence_path.
	runID := extractField(output, "run_id")
	require.NotEmpty(t, runID, "failed run must print run_id")

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "failed run must print evidence_path")

	// Verify the evidence directory exists and contains manifest.json.
	catCode, _, catErr := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "manifest.json")})
	require.NoError(t, catErr)
	require.Equal(t, 0, catCode, "evidence_path must contain manifest.json")

	// Success-only fields must NOT be printed for failed runs.
	require.NotContains(t, output, "workspace_path:")
	require.NotContains(t, output, "container_name:")
	require.NotContains(t, output, "attach:")
	require.NotContains(t, output, "promote:")
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

func TestE2E_IndexAppendFailureEmitsWarning(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")

	// Place a directory at index.jsonl path so AppendIndex's OpenFile fails
	// while the rest of the run succeeds normally.
	indexPath := filepath.Join(repoPath, ".tessariq", "runs", "index.jsonl")
	code, out, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("rm -f %s && mkdir %s", indexPath, indexPath)})
	require.NoError(t, err)
	require.Equal(t, 0, code, "index.jsonl dir setup failed: %s", out)

	code, output := runTessariq(t, env, "claude", "")
	require.Equal(t, 0, code, "run should succeed despite index failure: %s", output)
	require.Contains(t, output, "warning: index entry skipped:",
		"expected index warning in output: %s", output)
}

func TestE2E_SymlinkToExternalTaskRejected(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		skipImage: true,
	})

	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	// Create an external file outside the repo and a symlink inside the repo.
	setupCmds := []string{
		fmt.Sprintf("printf '# External Task\\n' > %s/external.md", hostDir),
		fmt.Sprintf("ln -s %s/external.md %s/tasks/symlinked.md", hostDir, repoPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m 'add symlinked task'", repoPath),
	}
	for _, cmd := range setupCmds {
		execCmd(t, env, ctx, cmd, "symlink setup")
	}

	// Run tessariq with the symlinked task path — should fail before container start.
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --egress none tasks/symlinked.md",
			repoPath, homeDir, binPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "run should reject symlink to external file: %s", output)
	require.Contains(t, output, "outside the repository")
	require.NotContains(t, output, "run_id:",
		"must fail before evidence bootstrap: %s", output)
}

func TestE2E_ControlCharTaskPathRejected(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		skipImage: true,
	})

	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	// Create a task file whose name contains a newline byte — a classic
	// commit-trailer injection vector.
	setupCmds := []string{
		// printf resolves \n; use a dollar-quoted string inside sh -c so the
		// literal newline byte lands in the filename.
		fmt.Sprintf("printf '# Forged Task\\n' > %s/tasks/$'bad\\nname'.md", repoPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m 'add tampered task'", repoPath),
	}
	for _, cmd := range setupCmds {
		execCmd(t, env, ctx, cmd, "control-char setup")
	}

	// Run tessariq with the tampered task path — must fail before container
	// start. The invocation stays hand-rolled because the dollar-quoted task
	// path is the subject of the test; runTessariqNoImage hardcodes
	// tasks/sample.md.
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --egress none $'tasks/bad\\nname.md'",
			repoPath, homeDir, binPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "run should reject control-char task path: %s", output)
	require.Contains(t, output, "control characters")
	require.NotContains(t, output, "run_id:",
		"must fail before evidence bootstrap: %s", output)

	requireNoRunDirectory(t, env)
}

func TestE2E_DirtyRepoRejectedBeforeContainerStart(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// skipImage: the dirty-repo gate must reject before any container work.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		skipImage: true,
	})

	ctx := context.Background()
	repoPath := filepath.Join(env.Dir(), "repo")

	// Dirty the repository with an uncommitted change after init.
	execCmd(t, env, ctx,
		fmt.Sprintf("printf 'uncommitted\\n' > %s/dirty.txt", repoPath),
		"dirty repo setup")

	// Run tessariq against the dirty repo — must fail before container start.
	code, output := runTessariqNoImage(t, env, "--egress none")
	require.NotEqual(t, 0, code, "run should reject a dirty repository: %s", output)
	require.Contains(t, output, "repository is dirty")
	require.NotContains(t, output, "run_id:",
		"must fail before evidence bootstrap: %s", output)

	// Verify no new run directory was created.
	requireNoRunDirectory(t, env)
}

// requireNoContainerForEnv asserts that no tessariq-* container is left on the
// Docker daemon with a mount pointing into this test environment's host
// directory. Scoping by host directory keeps the assertion correct while e2e
// tests run in parallel, and it works for pre-bootstrap failures that never mint
// a run id.
//
// This is a leak detector, not a proof that nothing was ever created. It runs
// after the tessariq process has exited, so any container the process created
// and then removed via `docker rm -f` is invisible; so is any container without
// bind mounts, which includes the Squid proxy container. Callers therefore must
// both:
//
//   - pin how far the run progressed — via requireNoRunDirectory (no run id
//     minted, so no tessariq-<run_id> name can exist) or via
//     requireEvidenceFileAbsent on runtime.json, which run.go writes immediately
//     before creating the agent container; and
//   - avoid --egress proxy/auto, because proxy topology setup creates a
//     tessariq-squid-<run_id> container before auth discovery runs.
func requireNoContainerForEnv(t *testing.T, env *containers.RunEnv) {
	t.Helper()
	ctx := context.Background()
	// `docker ps` failure exits non-zero so daemon trouble fails the test loudly
	// instead of reading as "nothing found". A `docker inspect` failure means the
	// container vanished mid-scan, which is a benign race, so it is skipped. The
	// loop's own status comes from `case`, which succeeds whether or not it
	// matches.
	script := fmt.Sprintf(
		"names=$(docker ps -a --filter name=tessariq- --format '{{.Names}}') || exit 1\n"+
			"for c in $names; do\n"+
			"  src=$(docker inspect --format '{{range .Mounts}}{{.Source}} {{end}}' \"$c\" 2>/dev/null) || continue\n"+
			"  case \"$src\" in *'%s'*) echo \"$c\" ;; esac\n"+
			"done\n", env.Dir())
	code, out, err := env.Exec(ctx, []string{"sh", "-c", script})
	require.NoError(t, err)
	require.Equal(t, 0, code, "container scan failed: %s", out)
	require.Empty(t, strings.TrimSpace(out),
		"no tessariq container should have been left behind, found: %s", out)
}

// requireEvidenceFileAbsent asserts an evidence artifact was never written,
// which pins how far a failed run progressed.
func requireEvidenceFileAbsent(t *testing.T, env *containers.RunEnv, evidenceDir, name string) {
	t.Helper()
	ctx := context.Background()
	code, out, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("test -e %s", filepath.Join(evidenceDir, name))})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "%s must not exist: %s", name, out)
}

// requireNoRunDirectory asserts the repository holds no evidence run directory,
// proving the failure landed before evidence bootstrap minted a run id.
// `tessariq init` leaves .tessariq/runs empty, so any entry other than
// index.jsonl means a run id was minted.
func requireNoRunDirectory(t *testing.T, env *containers.RunEnv) {
	t.Helper()
	ctx := context.Background()
	repoPath := filepath.Join(env.Dir(), "repo")
	_, out, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("ls -1 %s/.tessariq/runs 2>/dev/null | grep -v '^index.jsonl$' || true", repoPath)})
	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(out),
		"no evidence directory should be created, found: %s", out)
}

// TestE2E_ClaudeCodeMissingCredentialsFailsBeforeContainerStart covers manual
// case T4.8: Claude Code selected with no local credentials must name the
// missing auth and the fix instead of surfacing a raw I/O error.
func TestE2E_ClaudeCodeMissingCredentialsFailsBeforeContainerStart(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// No ~/.claude/.credentials.json and no ~/.claude.json.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{skipAuth: true, skipImage: true})

	// --egress none: proxy topology setup precedes auth discovery in run.go and
	// would create a tessariq-squid-<run_id> container, so the no-container
	// assertion below only holds for a non-proxy mode. The missing-credentials
	// failure is independent of the egress mode.
	code, output := runTessariqNoImage(t, env, "--egress none --no-update-agent")
	require.NotEqual(t, 0, code, "run must fail without Claude Code credentials: %s", output)

	// Failure UX: name the problem and the fix, not a raw filesystem error.
	require.Contains(t, output, "auth files or directories for claude-code were not found")
	require.Contains(t, output, "authenticate claude-code locally first")
	require.NotContains(t, output, "no such file or directory")

	// Auth discovery runs after evidence bootstrap but before the image probe,
	// so runtime.json must be absent and no container may exist.
	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "post-bootstrap failure must print evidence_path")
	requireEvidenceFileAbsent(t, env, evidencePath, "runtime.json")
	requireNoContainerForEnv(t, env)
}

// TestE2E_OpenCodeMissingAuthFileFailsWithActionableGuidance covers manual case
// T4.9 and regresses BUG-010: an absent OpenCode auth.json must not leak the
// raw read error from provider resolution under --egress auto.
func TestE2E_OpenCodeMissingAuthFileFailsWithActionableGuidance(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// No ~/.local/share/opencode/auth.json.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		binaryName: "opencode",
		skipAuth:   true,
		skipImage:  true,
	})

	code, output := runTessariqNoImage(t, env, "--agent opencode --egress auto --no-update-agent")
	require.NotEqual(t, 0, code, "run must fail without OpenCode auth.json: %s", output)

	require.Contains(t, output, "auth files or directories for opencode were not found")
	require.Contains(t, output, "authenticate opencode locally first")
	require.NotContains(t, output, "no such file or directory",
		"BUG-010 regression: raw filesystem error must not surface: %s", output)
	require.NotContains(t, output, "read auth file",
		"BUG-010 regression: provider-resolution internals must not surface: %s", output)

	// Provider resolution runs before evidence bootstrap, so no run id exists
	// and therefore no tessariq-<run_id> container can have been created.
	require.NotContains(t, output, "run_id:",
		"must fail before evidence bootstrap: %s", output)
	requireNoRunDirectory(t, env)
	requireNoContainerForEnv(t, env)
}

// TestE2E_UnknownModelProviderFailsWithEgressGuidance covers manual case T4.12:
// an unrecognized --model provider prefix under --egress auto must name the
// prefix and point at the two escape hatches.
func TestE2E_UnknownModelProviderFailsWithEgressGuidance(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// auth.json present but carrying no provider, so resolution falls through to
	// the --model prefix.
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{
		binaryName: "opencode",
		skipImage:  true,
		authFn: func(homeDir string) []string {
			return []string{
				fmt.Sprintf("mkdir -p %s/.local/share/opencode", homeDir),
				fmt.Sprintf(`printf '{"token":"fake"}' > %s/.local/share/opencode/auth.json`, homeDir),
			}
		},
	})

	code, output := runTessariqNoImage(t, env,
		"--agent opencode --model notaprovider/x --egress auto --no-update-agent")
	require.NotEqual(t, 0, code, "run must fail for an unknown model provider: %s", output)

	require.Contains(t, output, `unknown model provider "notaprovider"`,
		"error must name the unrecognized prefix: %s", output)
	require.Contains(t, output, "--egress-allow", "error must suggest --egress-allow: %s", output)
	require.Contains(t, output, "--egress open", "error must suggest --egress open: %s", output)

	require.NotContains(t, output, "run_id:",
		"must fail before evidence bootstrap: %s", output)
	requireNoRunDirectory(t, env)
	requireNoContainerForEnv(t, env)
}

func TestE2E_EgressNone_NoNetworkAccess(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// Agent script tries to reach an external host and writes the result.
	// With --net none only loopback is available, so wget must fail.
	script := `wget --timeout=2 http://1.1.1.1/ -O /dev/null 2>/work/net_result.txt; exit 0`
	env := setupRunEnvWithScript(t, bin, "claude", script)

	code, output := runTessariq(t, env, "claude", "--egress none")
	require.Equal(t, 0, code, "run should succeed (agent exits 0): %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	ctx := context.Background()

	// Verify evidence is written normally.
	catCode, statusData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "status.json must exist")

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(statusData), &status))
	require.Equal(t, "success", status["state"])

	// Read the network result written by the agent script.
	wsPath := extractField(output, "workspace_path")
	require.NotEmpty(t, wsPath)
	catCode, netResult, err := env.Exec(ctx, []string{"cat", filepath.Join(wsPath, "net_result.txt")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "net_result.txt must exist in workspace")

	// With --net none the connection must fail — wget reports "Network is unreachable"
	// or "bad address" (DNS failure) depending on the BusyBox version.
	netResultLower := strings.ToLower(netResult)
	networkBlocked := strings.Contains(netResultLower, "network is unreachable") ||
		strings.Contains(netResultLower, "bad address") ||
		strings.Contains(netResultLower, "network unreachable") ||
		strings.Contains(netResultLower, "can't connect")
	require.True(t, networkBlocked,
		"container must have no network access under --egress none, but wget output was: %s", netResult)
}

func TestE2E_FailedAgentExitsNonZero(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 1)

	code, output := runTessariq(t, env, "claude", "")
	require.NotEqual(t, 0, code, "failed run must exit non-zero")

	require.Contains(t, output, "state: failed")
	require.Contains(t, output, "run_id:")
	require.Contains(t, output, "evidence_path:")
	require.NotContains(t, output, "promote:")
	require.NotContains(t, output, "attach:")

	// Verify status.json records the failure.
	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()
	catCode, statusData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "status.json must exist")

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(statusData), &status))
	require.Equal(t, "failed", status["state"])
	require.Equal(t, float64(1), status["exit_code"])
}

func TestE2E_RunAttachSuppressesHintAndWritesEvidence(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	// Run with --attach in a non-TTY container. tmux attach will fail because
	// there is no terminal, but the runner itself should succeed and evidence
	// should be written.
	code, output := runTessariq(t, env, "claude", "--attach")

	// The command exits non-zero because tmux attach fails without a terminal,
	// but evidence artifacts are still produced because the run succeeded.
	require.NotEqual(t, 0, code, "run --attach should fail in non-TTY: %s", output)
	require.Contains(t, output, "attach to run session", "error should mention attach failure")

	// The "attach:" hint line must NOT appear because --attach was used.
	require.NotContains(t, output, "attach: tessariq attach ")

	// Evidence must still be accessible.
	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	ctx := context.Background()
	catCode, statusData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "status.json must exist")

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(statusData), &status))
	require.Equal(t, "success", status["state"], "run itself should succeed even when attach fails")
}

func TestE2E_TimedOutRunExitsNonZero(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "sleep 300")

	code, output := runTessariq(t, env, "claude", "--timeout 3s --grace 1s")
	require.NotEqual(t, 0, code, "timed-out run must exit non-zero")

	require.Contains(t, output, "state: timeout")
	require.Contains(t, output, "run_id:")
	require.Contains(t, output, "evidence_path:")
	require.NotContains(t, output, "promote:")

	// Verify status.json records the timeout.
	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()
	catCode, statusData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "status.json must exist")

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(statusData), &status))
	require.Equal(t, "timeout", status["state"])
	require.Equal(t, true, status["timed_out"])
}

// T-091: a hung pre-hook must not outlive the run --timeout. The CLI
// must exit non-zero, status.json must record StateTimeout, timeout.flag
// must be present, and the wall-clock duration must stay within the
// budget plus the grace period.
func TestE2E_PreHookTimeoutWritesEvidence(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	start := time.Now()
	code, output := runTessariq(t, env, "claude",
		"--timeout 3s --grace 1s --pre 'sleep 120'")
	elapsed := time.Since(start)

	require.NotEqual(t, 0, code, "pre-hook timeout must exit non-zero")
	require.Contains(t, output, "state: timeout")
	require.Contains(t, output, "evidence_path:")
	require.NotContains(t, output, "promote:")
	require.Less(t, elapsed, 30*time.Second,
		"pre-hook must be killed inside the budget; elapsed %s", elapsed)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()
	catCode, statusData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "status.json must exist after pre-hook timeout")

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(statusData), &status))
	require.Equal(t, "timeout", status["state"])
	require.Equal(t, true, status["timed_out"])

	flagCode, _, err := env.Exec(ctx, []string{"test", "-f", filepath.Join(evidencePath, "timeout.flag")})
	require.NoError(t, err)
	require.Equal(t, 0, flagCode, "timeout.flag must be written on pre-hook timeout")

	logCode, runnerLog, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "runner.log")})
	require.NoError(t, err)
	require.Equal(t, 0, logCode, "runner.log must exist")
	require.Contains(t, runnerLog, "phase=pre-hook",
		"runner.log must tag the timed-out phase")
}

// T-115: pre and verify hooks must execute with the repository root as their
// working directory, not the evidence directory. T-072 fixed this; nothing
// asserted it, so a regression would silently break every relative-path
// project command (`go test ./...`, `ls tasks/sample.md`) and force callers
// back to an inline `cd` workaround. Hook stdout/stderr must reach runner.log.
func TestE2E_HooksRunFromRepoRootAndCaptureOutput(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	repoPath := filepath.Join(env.Dir(), "repo")

	// Resolve the repo root the same way the hook shell will, so a symlinked
	// TMPDIR cannot make the comparison fail for the wrong reason.
	rootCode, wantRoot, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && pwd -P", repoPath)})
	require.NoError(t, err)
	require.Equal(t, 0, rootCode, "resolve repo root")
	require.NotEmpty(t, wantRoot)

	code, output := runTessariq(t, env, "claude", joinFlags(
		`--pre 'echo pre-cwd=$(pwd -P)'`,
		`--pre 'ls tasks/sample.md && echo pre-relative-ok'`,
		`--verify 'echo verify-cwd=$(pwd -P)'`,
		`--verify 'ls tasks/sample.md && echo verify-relative-ok'`,
	))
	require.Equal(t, 0, code, "run with repo-root hooks must succeed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	logCode, runnerLog, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "runner.log")})
	require.NoError(t, err)
	require.Equal(t, 0, logCode, "runner.log must exist")

	// Both phases carry the same contract; the trailing newline anchors the
	// cwd match so a nested directory cannot satisfy it by prefix.
	for _, phase := range []string{"pre", "verify"} {
		require.Contains(t, runnerLog, phase+"-cwd="+wantRoot+"\n",
			"%s-hook must run from the repository root", phase)
		require.NotContains(t, runnerLog, phase+"-cwd="+evidencePath,
			"%s-hook must not run from the evidence directory", phase)
		require.Contains(t, runnerLog, phase+"-relative-ok",
			"repo-relative path must resolve in a %s-hook without an inline cd", phase)
	}
}

// T-115: a failing --verify hook must fail the run — the CLI exits non-zero,
// status.json records StateFailed rather than success, and the failure is
// attributable in runner.log. Without this, a green verify gate could be
// silently ignored and a broken run would still look promotable.
func TestE2E_VerifyHookFailureFailsRun(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	code, output := runTessariq(t, env, "claude", `--verify 'echo verify-hook-ran; exit 3'`)
	require.NotEqual(t, 0, code, "failing verify hook must exit non-zero")
	require.Contains(t, output, "state: failed")
	require.NotContains(t, output, "promote:")

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()
	catCode, statusData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "status.json must exist after verify-hook failure")

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(statusData), &status))
	require.Equal(t, "failed", status["state"])
	require.Equal(t, float64(1), status["exit_code"])
	require.Equal(t, false, status["timed_out"])

	logCode, runnerLog, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "runner.log")})
	require.NoError(t, err)
	require.Equal(t, 0, logCode, "runner.log must exist")
	require.Contains(t, runnerLog, "verify-hook-ran",
		"verify-hook output must be captured in runner.log")
	require.Contains(t, runnerLog, "verify-hook failed",
		"runner.log must attribute the failure to the verify phase")
}

func TestE2E_AgentUpdate_SkipFlagBypassesInitPhase(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	// runTessariq already injects --no-update-agent as the suite-wide
	// default; this test still exercises the explicit flag form to guard
	// against a regression where the flag itself stops being honored.
	code, output := runTessariqRaw(t, env, "claude", joinFlags("--no-update-agent"))
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()

	catCode, runtimeData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "runtime.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "runtime.json must exist")

	var runtimeInfo adapter.RuntimeInfo
	require.NoError(t, json.Unmarshal([]byte(runtimeData), &runtimeInfo))

	// --no-update-agent must not produce an agent_update field.
	require.Nil(t, runtimeInfo.AgentUpdate,
		"agent_update must be nil when --no-update-agent is used")
}

func TestE2E_AgentUpdate_FallbackRecordsEvidence(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	// This test deliberately exercises the agent-update init container
	// (via runTessariqWithUpdate). The rest of the e2e suite goes through
	// runTessariq which skips the update phase. The init container will
	// fail because npm is not in the test alpine image, triggering the
	// fallback-to-baked path this test verifies.
	code, output := runTessariqWithUpdate(t, env, "claude", "")
	require.Equal(t, 0, code, "run must succeed even when agent update fails: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath)

	ctx := context.Background()

	catCode, runtimeData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "runtime.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "runtime.json must exist")

	var runtimeInfo adapter.RuntimeInfo
	require.NoError(t, json.Unmarshal([]byte(runtimeData), &runtimeInfo))

	// Agent update must be attempted and recorded as failure.
	require.NotNil(t, runtimeInfo.AgentUpdate,
		"agent_update must be present when update was attempted")
	require.True(t, runtimeInfo.AgentUpdate.Attempted)
	require.False(t, runtimeInfo.AgentUpdate.Success,
		"update must fail in test environment (no npm)")
	require.NotEmpty(t, runtimeInfo.AgentUpdate.Error)
	require.Greater(t, runtimeInfo.AgentUpdate.ElapsedMs, int64(0))
}

// interruptedRun holds the observations collected while interrupting a live
// run: the CLI exit code, the run id resolved from the evidence tree, and the
// combined CLI output.
type interruptedRun struct {
	exitCode int
	runID    string
	output   string
}

// interruptAgentMarker is printed by the fake agent before it sleeps so the
// test can prove the agent container really was live (and its output really
// was captured into run.log) before the interrupt is delivered.
const interruptAgentMarker = "AGENT_RUNNING_MARKER"

// runTessariqAndInterrupt starts `tessariq run` inside the e2e container,
// waits until the per-run agent container is live and its output has reached
// run.log, then delivers SIGINT to the CLI process and reports its exit code.
//
// The signal targets the CLI process inside the container (not the test
// process), and the wait loop keys off the deterministic
// tessariq-<run_id> container name so the interrupt window cannot be hit
// vacuously before the container exists.
func runTessariqAndInterrupt(t *testing.T, env *containers.RunEnv, binaryName, extraFlags string) interruptedRun {
	t.Helper()

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")
	runsDir := filepath.Join(repoPath, ".tessariq", "runs")
	flags := joinFlags(testAgentImageFlag(t, binaryName), "--no-update-agent", extraFlags)

	// The wait after SIGINT is bounded so a CLI that ignores the signal
	// surfaces as a clean assertion failure instead of hanging the harness
	// (and with it the whole go test run) until the package timeout. The
	// bound is 240s, comfortably above the worst-case shutdown budget:
	// --grace (3s here) + the 5s SIGKILL wait + the 30s container-cleanup
	// bound (runner.cleanupTimeout) + the 60s proxy-teardown bound
	// (proxyTeardownTimeout). Keeping the slack wide matters because this
	// test shares the host Docker daemon with the rest of the parallel e2e
	// suite.
	script := fmt.Sprintf(`
cd %[1]s
HOME=%[2]s %[3]s run %[4]s tasks/sample.md > /work/interrupt.out 2>&1 &
pid=$!
rid=""
live=no
i=0
while [ $i -lt 600 ]; do
  if [ -z "$rid" ]; then
    rid=$(ls -1 %[5]s 2>/dev/null | while read -r n; do [ -d %[5]s/"$n" ] && echo "$n"; done | head -n 1)
  fi
  if [ -n "$rid" ] &&
     [ "$(docker inspect -f '{{.State.Running}}' tessariq-$rid 2>/dev/null)" = "true" ] &&
     grep -q %[6]s %[5]s/"$rid"/run.log 2>/dev/null; then
    live=yes
    break
  fi
  sleep 0.2
  i=$((i+1))
done
echo "RUN_ID: $rid"
echo "CONTAINER_LIVE: $live"
kill -INT $pid
j=0
while kill -0 $pid 2>/dev/null && [ $j -lt 1200 ]; do
  sleep 0.2
  j=$((j+1))
done
if kill -0 $pid 2>/dev/null; then
  echo "CLI_EXITED: no"
  echo "EXIT_CODE: 0"
  kill -KILL $pid 2>/dev/null
  wait $pid 2>/dev/null
else
  wait $pid
  rc=$?
  echo "CLI_EXITED: yes"
  echo "EXIT_CODE: $rc"
fi
exit 0
`, repoPath, homeDir, binPath, flags, runsDir, interruptAgentMarker)

	ctx := context.Background()
	code, out, err := env.Exec(ctx, []string{"sh", "-c", script})
	require.NoError(t, err)
	require.Equal(t, 0, code, "interrupt harness must complete: %s", out)

	require.Equal(t, "yes", extractField(out, "CONTAINER_LIVE"),
		"agent container never became live before the interrupt: %s", out)
	require.Equal(t, "yes", extractField(out, "CLI_EXITED"),
		"CLI must exit after SIGINT instead of ignoring it: %s", out)

	runID := extractField(out, "RUN_ID")
	require.NotEmpty(t, runID, "run id must be resolvable from the evidence tree: %s", out)

	exitCode, err := strconv.Atoi(extractField(out, "EXIT_CODE"))
	require.NoError(t, err, "interrupt harness must report a CLI exit code: %s", out)

	return interruptedRun{
		exitCode: exitCode,
		runID:    runID,
		output:   readFileInEnv(t, env, "/work/interrupt.out"),
	}
}

// TestE2E_InterruptedProxyRunCleansUpAndRecordsTerminalState covers manual
// case T4.22 and guards the BUG-048 regression: a Ctrl-C on `tessariq run`
// must leave no agent container, no Squid container, no per-run network, and
// no worktree behind, and must record a terminal, non-running state that
// agrees with the non-zero CLI exit code.
func TestE2E_InterruptedProxyRunCleansUpAndRecordsTerminalState(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)

	// The agent announces itself and then sleeps well past the interrupt
	// window so the SIGINT lands while the container is genuinely running.
	env := setupRunEnvWithScript(t, bin, "claude",
		fmt.Sprintf("echo %s\nsleep 600", interruptAgentMarker))

	// A short grace keeps the SIGTERM -> SIGKILL escalation quick; the
	// default 30s would only pad the test.
	res := runTessariqAndInterrupt(t, env, "claude", "--egress proxy --grace 3s")
	require.NotEqual(t, 0, res.exitCode,
		"interrupted run must exit non-zero: %s", res.output)

	ctx := context.Background()
	evidencePath := filepath.Join(env.Dir(), "repo", ".tessariq", "runs", res.runID)

	// status.json must record a terminal, non-running state consistent with
	// the non-zero CLI exit code (BUG-048 regression).
	statusData := readFileInEnv(t, env, filepath.Join(evidencePath, "status.json"))

	var status runner.Status
	require.NoError(t, json.Unmarshal([]byte(statusData), &status))
	require.NotEqual(t, runner.StateRunning, status.State,
		"interrupted run must not stay running (BUG-048)")
	require.True(t, status.State.IsTerminal(), "recorded state must be terminal, got %q", status.State)
	require.NotEqual(t, runner.StateSuccess, status.State,
		"non-zero CLI exit must not be paired with a success state")
	require.Equal(t, runner.StateInterrupted, status.State,
		"SIGINT must map to the interrupted state")
	require.NotEmpty(t, status.FinishedAt, "terminal status must record finished_at")

	// No leftover agent container, Squid container, or per-run network.
	for _, name := range []string{"tessariq-" + res.runID, "tessariq-squid-" + res.runID} {
		_, out, err := env.Exec(ctx, []string{"sh", "-c",
			fmt.Sprintf("docker ps -a --format '{{.Names}}' --filter name=^%s$", name)})
		require.NoError(t, err)
		require.Empty(t, strings.TrimSpace(out), "container %s must not survive the interrupt", name)
	}
	netName := "tessariq-net-" + res.runID
	_, netOut, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("docker network ls --format '{{.Name}}' --filter name=^%s$", netName)})
	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(netOut), "network %s must not survive the interrupt", netName)

	// Catch-all for any other container this run's environment produced —
	// the name-based checks above only cover the two anticipated names.
	requireNoContainerForEnv(t, env)

	// The worktree must be gone from disk and from git's worktree list.
	requireWorktreeRemoved(t, env, res.runID)

	// Terminal evidence must survive container removal. Note the direction
	// of the guarantee: the runner deliberately runs container cleanup
	// (docker rm -f) *before* writing the final status.json — see the
	// comment at internal/runner/runner.go:219 and
	// TestRunner_CleanupCalledBeforeStatusWrite — so this is not an
	// evidence-then-removal ordering check. What it proves is that removing
	// the container destroys none of the evidence: the full artifact set is
	// on disk afterwards, and run.log still holds the agent output that was
	// streamed out of the container while it was alive.
	for _, name := range []string{
		"manifest.json", "status.json", "agent.json", "runtime.json",
		"workspace.json", "run.log", "runner.log", "task.md",
		"egress.compiled.yaml", "egress.events.jsonl",
	} {
		code, out, execErr := env.Exec(ctx, []string{"sh", "-c",
			fmt.Sprintf("test -s %s", filepath.Join(evidencePath, name))})
		require.NoError(t, execErr)
		require.Equal(t, 0, code,
			"%s must exist and be non-empty after container removal: %s", name, out)
	}

	logData := readFileInEnv(t, env, filepath.Join(evidencePath, "run.log"))
	require.Contains(t, logData, interruptAgentMarker,
		"agent output captured before the interrupt must persist after container removal")
}
