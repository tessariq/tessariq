//go:build manual_test

package claudecode_test

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

// MT-006: Full CLI invocation inside a container.
//
// Acceptance criteria tested:
//   - adapter.json records adapter=claude-code and the resolved image value.
//   - The adapter integrates cleanly with the run lifecycle.
//
// Run with: go test -tags=manual_test ./internal/adapter/claudecode/ -run TestManual_FullCLIRun -v -count=1
func TestManual_FullCLIRun(t *testing.T) {
	ctx := context.Background()

	// Build a static binary for Alpine.
	modRoot := findModRoot(t)
	bin := filepath.Join(t.TempDir(), "tessariq")
	build := exec.Command("go", "build", "-o", bin, "./cmd/tessariq")
	build.Dir = modRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build failed: %s", out)

	// Start a RunEnv container with tmux, git, and fake claude (exit 0).
	env, err := containers.StartRunEnv(ctx, t, 0)
	require.NoError(t, err, "StartRunEnv failed")

	// Copy binary into bind-mount.
	binData, err := os.ReadFile(bin)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(env.Dir(), "tessariq"), binData, 0o755))

	// Initialise a git repo with a task file inside the container.
	repoDir := "/work/repo"
	setup := []string{
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
	for _, cmd := range setup {
		code, stdout, err := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, err, "setup %q: %s", cmd, stdout)
		require.Equal(t, 0, code, "setup %q exited %d: %s", cmd, code, stdout)
	}

	// Run tessariq run inside the container.
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && /work/tessariq run tasks/sample.md"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "tessariq run failed: %s", output)

	// Verify all expected output fields are present.
	for _, field := range []string{"run_id", "evidence_path", "workspace_path", "container_name", "attach", "promote"} {
		require.Contains(t, output, field+": ", "output missing %q", field)
	}

	// Extract evidence_path.
	evidencePath := extractFieldFromOutput(t, output, "evidence_path")

	// Verify adapter.json exists and has correct content.
	catCode, adapterData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "adapter.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "adapter.json must exist in evidence dir")

	var info adapter.Info
	require.NoError(t, json.Unmarshal([]byte(adapterData), &info))
	require.Equal(t, 1, info.SchemaVersion, "schema_version must be 1")
	require.Equal(t, "claude-code", info.Adapter, "adapter must be claude-code")
	require.NotEmpty(t, info.Image, "image must not be empty")
	require.NotNil(t, info.Requested, "requested must not be nil")
	require.NotNil(t, info.Applied, "applied must not be nil")

	// Verify status.json exists.
	statusCode, _, err := env.Exec(ctx, []string{"test", "-f", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, statusCode, "status.json must exist in evidence dir")

	// Verify manifest.json exists.
	manifestCode, _, err := env.Exec(ctx, []string{"test", "-f", filepath.Join(evidencePath, "manifest.json")})
	require.NoError(t, err)
	require.Equal(t, 0, manifestCode, "manifest.json must exist in evidence dir")

	t.Logf("PASS: adapter=%s, image=%s, schema_version=%d", info.Adapter, info.Image, info.SchemaVersion)
	t.Logf("PASS: evidence at %s contains adapter.json, status.json, manifest.json", evidencePath)
}

func findModRoot(t *testing.T) string {
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

func extractFieldFromOutput(t *testing.T, output, key string) string {
	t.Helper()
	prefix := key + ": "
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	t.Fatalf("field %q not found in output", key)
	return ""
}
