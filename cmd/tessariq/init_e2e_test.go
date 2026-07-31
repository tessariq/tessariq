//go:build e2e

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

// setupInitEnv creates a RunEnv for init-only e2e cases. Init needs neither the
// agent image nor agent auth, so both are skipped to keep these cases cheap.
// setupRunEnvCustom already runs `tessariq init` once in <hostDir>/repo, so that
// repository is the result of a first invocation.
func setupInitEnv(t *testing.T) *containers.RunEnv {
	t.Helper()
	return setupRunEnvCustom(t, buildBinary(t), e2eSetupOpts{skipImage: true, skipAuth: true})
}

// runInit executes `tessariq init` in repoPath inside the container and fails
// the test unless it exits 0.
func runInit(t *testing.T, env *containers.RunEnv, ctx context.Context, repoPath, label string) {
	t.Helper()
	hostDir := env.Dir()
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")
	execCmd(t, env, ctx, fmt.Sprintf("cd %s && HOME=%s %s init", repoPath, homeDir, binPath), label)
}

// statMode reads the octal permission bits of path from inside the container.
// The mode must be read in the container because the host umask does not apply
// to paths the CLI creates there.
func statMode(t *testing.T, env *containers.RunEnv, path string) string {
	t.Helper()
	code, out, err := env.Exec(context.Background(), []string{"stat", "-c", "%a", path})
	require.NoError(t, err)
	require.Equal(t, 0, code, "stat %s: %s", path, out)
	return strings.TrimSpace(out)
}

// gitignoreLines returns the lines of repoPath/.gitignore read from inside the
// container.
func gitignoreLines(t *testing.T, env *containers.RunEnv, repoPath string) []string {
	t.Helper()
	path := filepath.Join(repoPath, ".gitignore")
	code, out, err := env.Exec(context.Background(), []string{"cat", path})
	require.NoError(t, err)
	require.Equal(t, 0, code, "read %s: %s", path, out)
	return strings.Split(strings.TrimRight(out, "\n"), "\n")
}

// countGitignoreEntries counts exact `.tessariq/` lines in repoPath/.gitignore.
func countGitignoreEntries(t *testing.T, env *containers.RunEnv, repoPath string) int {
	t.Helper()
	n := 0
	for _, line := range gitignoreLines(t, env, repoPath) {
		if strings.TrimSpace(line) == ".tessariq/" {
			n++
		}
	}
	return n
}

// TestE2E_InitCreatesOwnerOnlyStateTree pins the permission contract on the
// runtime state tree: 0700 keeps other host users from enumerating live run IDs.
func TestE2E_InitCreatesOwnerOnlyStateTree(t *testing.T) {
	t.Parallel()
	env := setupInitEnv(t)
	repoPath := filepath.Join(env.Dir(), "repo")

	require.Equal(t, "700", statMode(t, env, filepath.Join(repoPath, ".tessariq")),
		".tessariq must be owner-only")
	require.Equal(t, "700", statMode(t, env, filepath.Join(repoPath, ".tessariq", "runs")),
		".tessariq/runs must be owner-only")
}

// TestE2E_InitAddsGitignoreEntryWithoutDuplicating asserts init adds the
// `.tessariq/` ignore entry, and that a repository already carrying the entry
// keeps exactly one and retains its other rules.
func TestE2E_InitAddsGitignoreEntryWithoutDuplicating(t *testing.T) {
	t.Parallel()
	env := setupInitEnv(t)
	ctx := context.Background()
	hostDir := env.Dir()

	// The repo initialised by setupRunEnvCustom had no .gitignore before init.
	require.Equal(t, 1, countGitignoreEntries(t, env, filepath.Join(hostDir, "repo")),
		"init must add exactly one .tessariq/ entry")

	// A repository that already carries the entry must not gain a duplicate.
	preseeded := filepath.Join(hostDir, "preseeded")
	for _, cmd := range []string{
		fmt.Sprintf("git init %s", preseeded),
		fmt.Sprintf("printf 'node_modules/\\n.tessariq/\\n' > %s/.gitignore", preseeded),
		fmt.Sprintf("git -C %s add -A && git -C %s commit -m initial", preseeded, preseeded),
	} {
		execCmd(t, env, ctx, cmd, "preseeded repo setup")
	}

	runInit(t, env, ctx, preseeded, "init on preseeded repo")

	require.Equal(t, 1, countGitignoreEntries(t, env, preseeded),
		"existing .tessariq/ entry must not be duplicated")
	require.Contains(t, gitignoreLines(t, env, preseeded), "node_modules/",
		"pre-existing ignore rules must be preserved")
}

// TestE2E_InitIsIdempotentAndTightensPermissions asserts a second init succeeds,
// leaves repository content untouched, and re-tightens a state tree that was
// loosened beyond 0700.
func TestE2E_InitIsIdempotentAndTightensPermissions(t *testing.T) {
	t.Parallel()
	env := setupInitEnv(t)
	ctx := context.Background()
	repoPath := filepath.Join(env.Dir(), "repo")
	tessariqDir := filepath.Join(repoPath, ".tessariq")
	runsDir := filepath.Join(tessariqDir, "runs")

	// Loosen both directories and drop a marker so re-init cannot silently
	// recreate the tree instead of tightening it in place.
	marker := filepath.Join(runsDir, "marker")
	execCmd(t, env, ctx, fmt.Sprintf("printf 'keep\\n' > %s", marker), "marker setup")
	execCmd(t, env, ctx, fmt.Sprintf("chmod 0755 %s %s", tessariqDir, runsDir), "loosen perms")

	runInit(t, env, ctx, repoPath, "second init")

	require.Equal(t, "700", statMode(t, env, tessariqDir),
		"second init must tighten .tessariq back to 0700")
	require.Equal(t, "700", statMode(t, env, runsDir),
		"second init must tighten .tessariq/runs back to 0700")

	require.Equal(t, 1, countGitignoreEntries(t, env, repoPath),
		"second init must not duplicate the .gitignore entry")

	catCode, markerOut, err := env.Exec(ctx, []string{"cat", marker})
	require.NoError(t, err)
	require.Equal(t, 0, catCode, "existing state must survive re-init")
	require.Equal(t, "keep", strings.TrimSpace(markerOut))

	statusCode, status, err := env.Exec(ctx, []string{"git", "-C", repoPath, "status", "--porcelain"})
	require.NoError(t, err)
	require.Equal(t, 0, statusCode, "git status failed: %s", status)
	require.Empty(t, strings.TrimSpace(status),
		"second init must not change tracked repository content")
}
