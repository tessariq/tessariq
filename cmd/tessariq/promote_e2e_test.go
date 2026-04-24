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

func TestE2E_PromoteForgedEvidencePathShowsActionableGuidance(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{skipImage: true})

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	ctx := context.Background()

	// Write a forged index entry with an absolute external evidence path.
	cmd := fmt.Sprintf(`mkdir -p %s/.tessariq/runs && printf '{"run_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","created_at":"2026-01-01T00:00:00Z","task_path":"tasks/sample.md","task_title":"Forged Task","agent":"claude-code","workspace_mode":"worktree","state":"success","evidence_path":"/tmp/evil-evidence"}\n' > %s/.tessariq/runs/index.jsonl`, repoPath, repoPath)
	execCmd(t, env, ctx, cmd, "write forged index")

	code, output := runPromote(t, env, "last", "")
	require.NotEqual(t, 0, code, "promote should fail with forged evidence path")
	require.Contains(t, output, "outside the repository")
}

func TestE2E_PromoteLastNResolvesUniqueRuns(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo promoted > /work/promoted.txt; exit 0")

	// Run A: a complete run that produces code changes.
	runCodeA, runOutputA := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCodeA, "run A failed: %s", runOutputA)
	runA := extractField(runOutputA, "run_id")
	require.NotEmpty(t, runA)

	// Run B: another complete run that produces code changes.
	// Must re-commit worktree changes so the repo is clean for the next run.
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	ctx := context.Background()
	execCmd(t, env, ctx, fmt.Sprintf("git -C %s add -A && git -C %s commit -m 'post A' --allow-empty", repoPath, repoPath), "commit after A")

	runCodeB, runOutputB := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCodeB, "run B failed: %s", runOutputB)
	runB := extractField(runOutputB, "run_id")
	require.NotEmpty(t, runB)
	require.NotEqual(t, runA, runB)

	// The index now has multiple lifecycle entries for each run (running + terminal).
	// promote last → should resolve to run B.
	// promote last-1 → should resolve to run A (previous unique run).
	promoteCode, promoteOutput := runPromote(t, env, "last-1", "")
	require.Equal(t, 0, promoteCode, "promote last-1 failed: %s", promoteOutput)
	require.Contains(t, promoteOutput, "branch: tessariq/"+runA,
		"last-1 must resolve to the previous unique run (A=%s), got: %s", runA, promoteOutput)
}

func TestE2E_PromoteTamperedManifestRejectedBeforeGitSideEffects(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo promoted > /work/promoted.txt; exit 0")

	runCode, runOutput := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCode, "run failed: %s", runOutput)

	runID := extractField(runOutput, "run_id")
	require.NotEmpty(t, runID)

	// Tamper with manifest.json: replace run_id with a different value.
	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	manifestPath := filepath.Join(repoPath, ".tessariq", "runs", runID, "manifest.json")
	tamperedRunID := "01BBBBBBBBBBBBBBBBBBBBBBBBB"
	tamperCmd := fmt.Sprintf(`sed -i 's/"run_id": "%s"/"run_id": "%s"/' %s`, runID, tamperedRunID, manifestPath)
	execCmd(t, env, ctx, tamperCmd, "tamper manifest")

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.NotEqual(t, 0, promoteCode, "promote should fail with tampered manifest: %s", promoteOutput)
	require.Contains(t, promoteOutput, "tampered")

	// Assert no branch was created.
	code, _, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("git -C %s show-ref --verify --quiet refs/heads/tessariq/%s", repoPath, runID)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "branch must not be created for tampered manifest")
}

func TestE2E_PromoteProxyRequiresEgressEvidence(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo promoted > /work/promoted.txt; exit 0")

	runCode, runOutput := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCode, "run failed: %s", runOutput)

	runID := extractField(runOutput, "run_id")
	require.NotEmpty(t, runID)

	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	evidenceDir := filepath.Join(repoPath, ".tessariq", "runs", runID)
	manifestPath := filepath.Join(evidenceDir, "manifest.json")
	runtimePath := filepath.Join(evidenceDir, "runtime.json")

	// Flip resolved_egress_mode to "proxy" in both manifest and runtime so
	// promote exercises the proxy completeness gate without a mismatch error.
	tamperManifest := fmt.Sprintf(`sed -i 's/"resolved_egress_mode": "[^"]*"/"resolved_egress_mode": "proxy"/' %s`, manifestPath)
	tamperRuntime := fmt.Sprintf(`sed -i 's/"resolved_egress_mode": "[^"]*"/"resolved_egress_mode": "proxy"/' %s`, runtimePath)
	execCmd(t, env, ctx, tamperManifest, "mark manifest as proxy mode")
	execCmd(t, env, ctx, tamperRuntime, "mark runtime as proxy mode")

	compiledPath := filepath.Join(evidenceDir, "egress.compiled.yaml")
	eventsPath := filepath.Join(evidenceDir, "egress.events.jsonl")

	// Write egress.compiled.yaml but leave egress.events.jsonl missing → promote
	// must refuse with the canonical "evidence is intact" guidance.
	execCmd(t, env, ctx, fmt.Sprintf("printf 'schema_version: 1\\n' > %s", compiledPath), "write compiled.yaml")

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.NotEqual(t, 0, promoteCode, "promote should fail when egress.events.jsonl is missing: %s", promoteOutput)
	require.Contains(t, promoteOutput, "egress.events.jsonl")
	require.Contains(t, promoteOutput, "evidence is intact")

	// Assert no branch was created on the failure path.
	code, _, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("git -C %s show-ref --verify --quiet refs/heads/tessariq/%s", repoPath, runID)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "branch must not be created when proxy evidence is incomplete")

	// Add the second required artifact and retry — promote should now succeed.
	execCmd(t, env, ctx, fmt.Sprintf(`printf '{"timestamp":"2026-01-01T00:00:00Z","host":"blocked.example.com","port":443,"action":"blocked","reason":"not_in_allowlist","squid_result":"TCP_DENIED/403"}\n' > %s`, eventsPath), "write events.jsonl")

	promoteCode, promoteOutput = runPromote(t, env, runID, "")
	require.Equal(t, 0, promoteCode, "promote should succeed when both proxy artifacts exist: %s", promoteOutput)
	require.Contains(t, promoteOutput, "branch: tessariq/"+runID)
}

func TestE2E_PromoteEgressModeTamperRejectedBeforeGitSideEffects(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "echo promoted > /work/promoted.txt; exit 0")

	runCode, runOutput := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCode, "run failed: %s", runOutput)

	runID := extractField(runOutput, "run_id")
	require.NotEmpty(t, runID)

	ctx := context.Background()
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	evidenceDir := filepath.Join(repoPath, ".tessariq", "runs", runID)
	manifestPath := filepath.Join(evidenceDir, "manifest.json")

	// Tamper manifest only: flip resolved_egress_mode from "open" to "proxy".
	// runtime.json still says "open" — the cross-check must catch the mismatch.
	tamperCmd := fmt.Sprintf(`sed -i 's/"resolved_egress_mode": "[^"]*"/"resolved_egress_mode": "proxy"/' %s`, manifestPath)
	execCmd(t, env, ctx, tamperCmd, "tamper manifest egress mode")

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.NotEqual(t, 0, promoteCode, "promote should fail with egress mode mismatch: %s", promoteOutput)
	require.Contains(t, promoteOutput, "tampered")

	// Assert no branch was created.
	code, _, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("git -C %s show-ref --verify --quiet refs/heads/tessariq/%s", repoPath, runID)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "branch must not be created when egress mode is tampered")
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
