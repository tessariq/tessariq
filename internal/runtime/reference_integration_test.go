//go:build integration

package runtime_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

func startRuntime(t *testing.T) *containers.RuntimeEnv {
	t.Helper()
	ctx := context.Background()
	env, err := containers.StartReferenceRuntime(ctx, t)
	require.NoError(t, err)
	return env
}

func TestReferenceRuntime_BaselineTools(t *testing.T) {
	t.Parallel()
	env := startRuntime(t)
	ctx := context.Background()

	// Tools that must be reachable via "which".
	whichTools := []string{
		"bash", "curl", "git", "jq", "rg",
		"zip", "unzip", "tar", "xz",
		"patch", "ps", "less", "ssh",
		"make", "gcc", "pkg-config",
		"python3", "pip3",
		"node", "npm", "corepack",
		"go",
	}

	for _, tool := range whichTools {
		code, out, err := env.Exec(ctx, []string{"which", tool})
		require.NoError(t, err, "which %s: exec error", tool)
		require.Equalf(t, 0, code, "which %s: expected exit 0, got %d (output: %s)", tool, code, out)
	}

	// Go version must contain "1.26".
	code, out, err := env.Exec(ctx, []string{"go", "version"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Contains(t, out, "go1.26", "go version output should contain go1.26")

	// Python venv must be usable.
	code, _, err = env.Exec(ctx, []string{"python3", "-m", "venv", "--help"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "python3 -m venv --help should exit 0")
}

func TestReferenceRuntime_NonRootUser(t *testing.T) {
	t.Parallel()
	env := startRuntime(t)
	ctx := context.Background()

	code, out, err := env.Exec(ctx, []string{"whoami"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Equal(t, "tessariq", out)

	code, out, err = env.Exec(ctx, []string{"id", "-u"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.NotEqual(t, "0", strings.TrimSpace(out), "user should not be root (uid 0)")
}

func TestReferenceRuntime_NoAgentBinaries(t *testing.T) {
	t.Parallel()
	env := startRuntime(t)
	ctx := context.Background()

	for _, binary := range []string{"claude", "opencode"} {
		code, _, err := env.Exec(ctx, []string{"which", binary})
		require.NoError(t, err, "which %s: exec error", binary)
		require.NotEqualf(t, 0, code, "which %s should fail (binary must not be bundled)", binary)
	}
}
