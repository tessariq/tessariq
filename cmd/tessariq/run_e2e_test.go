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
	t.Helper()

	ctx := context.Background()
	env, err := containers.StartRunEnv(ctx, t, claudeExitCode)
	require.NoError(t, err)

	// Copy the tessariq binary into the bind-mounted dir.
	binData, err := os.ReadFile(bin)
	require.NoError(t, err)
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

func TestE2E_AdapterJSONWritten(t *testing.T) {
	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && /work/tessariq run tasks/sample.md"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "run failed: %s", output)

	evidencePath := extractField(output, "evidence_path")
	require.NotEmpty(t, evidencePath, "evidence_path must be in output")

	// Read adapter.json from inside the container.
	catCode, adapterData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "adapter.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "adapter.json must exist")

	var info adapter.Info
	require.NoError(t, json.Unmarshal([]byte(adapterData), &info))
	require.Equal(t, 1, info.SchemaVersion)
	require.Equal(t, "claude-code", info.Adapter)
	require.NotEmpty(t, info.Image)
	require.NotNil(t, info.Requested)
	require.NotNil(t, info.Applied)
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
