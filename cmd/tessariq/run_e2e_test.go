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
	"github.com/tessariq/tessariq/internal/testutil"
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

// e2eSetupOpts controls which parts of the standard e2e setup are executed.
// Zero values produce the default behavior (build image, create auth, no extras).
type e2eSetupOpts struct {
	binaryName string                        // agent binary name; defaults to "claude"
	scriptBody string                        // fake agent script body; defaults to "exit 0"
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
				`cat > /work/Dockerfile.bare <<'DEOF'
FROM alpine:latest
RUN addgroup -S tessariq && adduser -S tessariq -G tessariq -h /home/tessariq
USER tessariq
WORKDIR /work
DEOF`,
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
RUN addgroup -S tessariq && adduser -S tessariq -G tessariq -h /home/tessariq
COPY fake-agent.sh /usr/local/bin/%s
RUN chmod +x /usr/local/bin/%s
USER tessariq
WORKDIR /work
DEOF`, opts.binaryName, opts.binaryName),
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
func execCmd(t *testing.T, env *containers.RunEnv, ctx context.Context, cmd, label string) {
	t.Helper()
	code, out, err := env.Exec(ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err, "%s %q: %s", label, cmd, out)
	require.Equal(t, 0, code, "%s %q exited %d: %s", label, cmd, code, out)
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

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	ctx := context.Background()
	_, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --egress open tasks/sample.md",
			repoPath, homeDir, binPath)})
	require.NoError(t, err)

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

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	ctx := context.Background()
	_, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --egress-allow api.example.com:443 tasks/sample.md",
			repoPath, homeDir, binPath)})
	require.NoError(t, err)

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

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && HOME=%s %s run --egress none tasks/sample.md",
			repoPath, homeDir, binPath)})
	require.NoError(t, err)
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
