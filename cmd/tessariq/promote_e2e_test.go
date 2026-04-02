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

func TestE2E_RunPromoteCreatesBranchAndCommit(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo promoted > /work/promoted.txt; exit 0")

	runCode, runOutput := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCode, "run failed: %s", runOutput)

	runID := extractField(runOutput, "run_id")
	require.NotEmpty(t, runID)

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.Equal(t, 0, promoteCode, "promote failed: %s", promoteOutput)
	require.Contains(t, promoteOutput, "branch: tessariq/"+runID)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	ctx := context.Background()

	code, logOut, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("git -C %s log -1 --format=%%B tessariq/%s", repoPath, runID)})
	require.NoError(t, err)
	require.Equal(t, 0, code, "git log failed: %s", logOut)
	require.Contains(t, logOut, "Sample Task")
	require.Contains(t, logOut, "Tessariq-Run: "+runID)

	code, showOut, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("git -C %s show --stat --format= tessariq/%s", repoPath, runID)})
	require.NoError(t, err)
	require.Equal(t, 0, code, "git show failed: %s", showOut)
	require.Contains(t, showOut, "promoted.txt")
}

func TestE2E_PromoteZeroDiffFailsWithoutBranch(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	runCode, runOutput := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCode, "run failed: %s", runOutput)

	runID := extractField(runOutput, "run_id")
	require.NotEmpty(t, runID)

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.NotEqual(t, 0, promoteCode, "promote should fail: %s", promoteOutput)
	require.Contains(t, strings.ToLower(promoteOutput), "no code changes")

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	ctx := context.Background()

	code, _, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("git -C %s show-ref --verify --quiet refs/heads/tessariq/%s", repoPath, runID)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "branch must not be created for zero-diff run")
}

func TestE2E_PromoteMissingGitShowsActionableGuidance(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo promoted > /work/promoted.txt; exit 0")

	runCode, runOutput := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCode, "run failed: %s", runOutput)

	runID := extractField(runOutput, "run_id")
	require.NotEmpty(t, runID)

	promoteCode, promoteOutput := runPromote(t, env, runID, "PATH=/no-such-bin")
	require.NotEqual(t, 0, promoteCode, "promote should fail when git is unavailable")
	require.Contains(t, promoteOutput, "required host prerequisite \"git\" is missing or unavailable")
	require.Contains(t, promoteOutput, "install or enable git, then retry")
}

func TestE2E_PromoteMissingDiffstatShowsActionableGuidance(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo promoted > /work/promoted.txt; exit 0")

	runCode, runOutput := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCode, "run failed: %s", runOutput)

	runID := extractField(runOutput, "run_id")
	require.NotEmpty(t, runID)

	// Remove diffstat.txt from the evidence directory before promoting.
	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	diffstatPath := filepath.Join(repoPath, ".tessariq", "runs", runID, "diffstat.txt")
	code, out, err := env.Exec(ctx, []string{"rm", "-f", diffstatPath})
	require.NoError(t, err)
	require.Equal(t, 0, code, "rm failed: %s", out)

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.NotEqual(t, 0, promoteCode, "promote should fail when diffstat.txt is missing")
	require.Contains(t, promoteOutput, "diffstat.txt")
	require.Contains(t, promoteOutput, "evidence is intact")
}

func TestE2E_PromoteLastFailsCleanlyWithIncompleteIndex(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{skipImage: true})

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	ctx := context.Background()

	// Write only incomplete index entries (missing required fields) inside the container.
	cmd := fmt.Sprintf(`mkdir -p %s/.tessariq/runs && printf '{"run_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","state":"success"}\n' > %s/.tessariq/runs/index.jsonl`, repoPath, repoPath)
	execCmd(t, env, ctx, cmd, "write corrupt index")

	code, output := runPromote(t, env, "last", "")
	require.NotEqual(t, 0, code, "promote should fail with incomplete index")
	require.Contains(t, output, "run index is empty")
}

func runPromote(t *testing.T, env *containers.RunEnv, runID, envPrefix string) (int, string) {
	t.Helper()

	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")
	prefix := fmt.Sprintf("HOME=%s", homeDir)
	if envPrefix != "" {
		prefix = envPrefix + " " + prefix
	}
	cmd := fmt.Sprintf("cd %s && %s %s promote %s", repoPath, prefix, binPath, runID)
	code, output, err := env.Exec(ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err)
	return code, output
}
