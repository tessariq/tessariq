//go:build manual_test

package opencode_test

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

func buildBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tessariq")
	build := exec.Command("go", "build", "-o", bin, "./cmd/tessariq")
	build.Dir = findModRoot(t)
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build failed: %s", out)
	return bin
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

const repoDir = "/work/repo"

func setupRunEnv(t *testing.T, bin string) *containers.RunEnv {
	t.Helper()
	ctx := context.Background()
	env, err := containers.StartRunEnvForBinary(ctx, t, "opencode", 0)
	require.NoError(t, err)

	binData, err := os.ReadFile(bin)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(env.Dir(), "tessariq"), binData, 0o755))

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
		code, stdout, err := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, err, "setup %q: %s", cmd, stdout)
		require.Equal(t, 0, code, "setup %q exited %d: %s", cmd, code, stdout)
	}
	return env
}

// MT-001: adapter.json records adapter=opencode and resolved image.
func TestManual_AdapterJSONRecordsOpenCode(t *testing.T) {
	bin := buildBin(t)
	env := setupRunEnv(t, bin)
	ctx := context.Background()

	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && /work/tessariq run --agent opencode tasks/sample.md"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "tessariq run failed: %s", output)

	evidencePath := extractFieldFromOutput(t, output, "evidence_path")

	catCode, adapterData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "adapter.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "adapter.json must exist")

	var info adapter.Info
	require.NoError(t, json.Unmarshal([]byte(adapterData), &info))
	require.Equal(t, 1, info.SchemaVersion, "schema_version must be 1")
	require.Equal(t, "opencode", info.Adapter, "adapter must be opencode")
	require.NotEmpty(t, info.Image, "image must not be empty")
	require.NotNil(t, info.Requested, "requested must not be nil")
	require.NotNil(t, info.Applied, "applied must not be nil")

	t.Logf("PASS: adapter=%s, image=%s, schema_version=%d", info.Adapter, info.Image, info.SchemaVersion)
}

// MT-004: Adapter integrates with run lifecycle (full CLI).
func TestManual_FullCLIRunLifecycle(t *testing.T) {
	bin := buildBin(t)
	env := setupRunEnv(t, bin)
	ctx := context.Background()

	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cd " + repoDir + " && /work/tessariq run --agent opencode tasks/sample.md"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "tessariq run failed: %s", output)

	// Verify all guidance fields present.
	for _, field := range []string{"run_id", "evidence_path", "workspace_path", "container_name", "attach", "promote"} {
		require.Contains(t, output, field+": ", "output missing %q", field)
	}

	evidencePath := extractFieldFromOutput(t, output, "evidence_path")

	// Verify adapter.json exists.
	catCode, adapterData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "adapter.json")})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "adapter.json must exist")

	var info adapter.Info
	require.NoError(t, json.Unmarshal([]byte(adapterData), &info))
	require.Equal(t, "opencode", info.Adapter)

	// Verify status.json exists.
	statusCode, _, err := env.Exec(ctx, []string{"test", "-f", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, statusCode, "status.json must exist")

	// Verify manifest.json exists.
	manifestCode, _, err := env.Exec(ctx, []string{"test", "-f", filepath.Join(evidencePath, "manifest.json")})
	require.NoError(t, err)
	require.Equal(t, 0, manifestCode, "manifest.json must exist")

	t.Logf("PASS: full lifecycle completed, evidence at %s", evidencePath)
}
