//go:build e2e

package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
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

func TestE2E_RunPromoteBinaryFileKeepsContentIntact(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	// A deterministic payload covering every octet value. The NUL bytes make git
	// classify the file as binary, so the diff is only representable with
	// `git diff --binary` (BUG-033).
	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i)
	}

	// The agent decodes the payload from base64 instead of writing escapes:
	// the fake-agent script is materialised through a printf format string, so
	// any backslash escape in the script body would be consumed before the
	// agent ever runs.
	script := fmt.Sprintf("echo %s | base64 -d > /work/asset.bin; exit 0",
		base64.StdEncoding.EncodeToString(payload))
	env := setupRunEnvWithScript(t, bin, "claude", script)

	runCode, runOutput := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCode, "run failed: %s", runOutput)

	runID := extractField(runOutput, "run_id")
	require.NotEmpty(t, runID)
	evidencePath := extractField(runOutput, "evidence_path")
	require.NotEmpty(t, evidencePath)

	// diff.patch must carry the binary content itself. Without --binary git
	// degrades to an unappliable "Binary files ... differ" stub and the file
	// is silently dropped at promote (BUG-033).
	patch := readFileInEnv(t, env, filepath.Join(evidencePath, "diff.patch"))
	require.Contains(t, patch, "asset.bin")
	require.Contains(t, patch, "GIT binary patch",
		"diff.patch must embed the binary payload, not a textual placeholder")
	require.NotContains(t, patch, "Binary files",
		"diff.patch must not degrade to an unappliable binary stub")

	stat := readFileInEnv(t, env, filepath.Join(evidencePath, "diffstat.txt"))
	require.Contains(t, stat, "asset.bin")
	require.Contains(t, stat, "Bin", "diffstat.txt must report the binary change")

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.Equal(t, 0, promoteCode, "promote failed: %s", promoteOutput)

	repoPath := filepath.Join(env.Dir(), "repo")
	blobRef := fmt.Sprintf("tessariq/%s:asset.bin", runID)

	// Size first: a patch that applies cleanly but truncates content would
	// otherwise only show up as an opaque checksum mismatch.
	size := gitOutput(t, env, repoPath, fmt.Sprintf("cat-file -s %s", blobRef))
	require.Equal(t, strconv.Itoa(len(payload)), size,
		"promoted blob must have the byte length the agent wrote")

	sum := sha256.Sum256(payload)
	promotedSum := gitOutput(t, env, repoPath,
		fmt.Sprintf("show %s | sha256sum | awk '{print $1}'", blobRef))
	require.Equal(t, hex.EncodeToString(sum[:]), promotedSum,
		"promoted blob must be byte-for-byte what the agent wrote")
}

func TestE2E_PromoteZeroDiffFailsWithoutBranch(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)

	runCode, runOutput := runTessariq(t, env, "claude", "--egress open")
	require.Equal(t, 0, runCode, "run failed: %s", runOutput)

	runID := extractField(runOutput, "run_id")
	require.NotEmpty(t, runID)

	repoPath := filepath.Join(env.Dir(), "repo")
	baseline := captureCleanBaseline(t, env, repoPath)

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.NotEqual(t, 0, promoteCode, "promote should fail: %s", promoteOutput)
	require.Contains(t, strings.ToLower(promoteOutput), "no code changes")

	requireGitStateUnchanged(t, env, repoPath, baseline)
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

	// Write only an incomplete index entry: every field except run_id and state
	// is left at its zero value, so ReadIndex drops it as incomplete.
	writeIndexEntries(t, env, run.IndexEntry{RunID: knownRunID, State: "success"})

	code, output := runPromote(t, env, "last", "")
	require.NotEqual(t, 0, code, "promote should fail with incomplete index")
	require.Contains(t, output, "run index is empty")
}

func TestE2E_PromoteForgedEvidencePathShowsActionableGuidance(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{skipImage: true})

	// Write a forged index entry with an absolute external evidence path.
	forged := indexEntryFixture(knownRunID, "success")
	forged.EvidencePath = "/tmp/evil-evidence"
	writeIndexEntries(t, env, forged)

	code, output := runPromote(t, env, "last", "")
	require.NotEqual(t, 0, code, "promote should fail with forged evidence path")
	require.Contains(t, output, "outside the repository")
}

func TestE2E_PromoteUnknownRunIDFailsWithoutBranch(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{skipImage: true})

	repoPath := filepath.Join(env.Dir(), "repo")

	// The known run's evidence directory is deliberately not created: resolving
	// a different, unknown ref must fail before any evidence is touched.
	writeIndexEntries(t, env, indexEntryFixture(knownRunID, "success"))
	baseline := captureCleanBaseline(t, env, repoPath)

	code, output := runPromote(t, env, unknownRunID, "")
	require.NotEqual(t, 0, code, "promote must fail for a run id that is not in the index")
	require.Contains(t, output, "run not found")
	require.Contains(t, output, unknownRunID)
	require.NotContains(t, output, "panic:", "an unresolvable ref must fail as an error, not a crash")
	requireGitStateUnchanged(t, env, repoPath, baseline)
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

	baseline := captureCleanBaseline(t, env, repoPath)

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.NotEqual(t, 0, promoteCode, "promote should fail with tampered manifest: %s", promoteOutput)
	require.Contains(t, promoteOutput, "tampered")

	requireGitStateUnchanged(t, env, repoPath, baseline)
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
	execCmd(t, env, ctx, fmt.Sprintf("printf 'schema_version: 1\\nallowlist_source: built_in\\ndestinations:\\n  - host: example.com\\n    port: 443\\n' > %s", compiledPath), "write compiled.yaml")

	baseline := captureCleanBaseline(t, env, repoPath)

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.NotEqual(t, 0, promoteCode, "promote should fail when egress.events.jsonl is missing: %s", promoteOutput)
	require.Contains(t, promoteOutput, "egress.events.jsonl")
	require.Contains(t, promoteOutput, "evidence is intact")

	requireGitStateUnchanged(t, env, repoPath, baseline)

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

	baseline := captureCleanBaseline(t, env, repoPath)

	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.NotEqual(t, 0, promoteCode, "promote should fail with egress mode mismatch: %s", promoteOutput)
	require.Contains(t, promoteOutput, "tampered")

	requireGitStateUnchanged(t, env, repoPath, baseline)
}

// TestE2E_PromoteRejectsMissingAndMalformedEvidenceBeforeGitSideEffects proves
// the evidence gate holds end to end for evidence that is simply absent or
// structurally unusable, not just tampered: a deleted status.json, an
// agent.json that is non-empty but unparseable, and an agent.json that parses
// yet carries none of the required fields. The last case is the BUG-063
// regression — non-empty JSON must not satisfy the completeness check on file
// size alone. Every case must be rejected before Git is touched.
func TestE2E_PromoteRejectsMissingAndMalformedEvidenceBeforeGitSideEffects(t *testing.T) {
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
	statusPath := filepath.Join(evidenceDir, "status.json")
	agentPath := filepath.Join(evidenceDir, "agent.json")
	backupDir := filepath.Join(hostDir, "evidence-backup")

	execCmd(t, env, ctx,
		fmt.Sprintf("mkdir -p %s && cp %s %s %s/", backupDir, statusPath, agentPath, backupDir),
		"back up evidence")

	baseline := captureCleanBaseline(t, env, repoPath)

	restoreCmd := fmt.Sprintf("cp %s/status.json %s/agent.json %s/", backupDir, backupDir, evidenceDir)

	tests := []struct {
		name       string
		tamperCmd  string
		wantOutput []string
	}{
		{
			name:       "status.json missing",
			tamperCmd:  fmt.Sprintf("rm -f %s", statusPath),
			wantOutput: []string{"incomplete evidence: status.json", "evidence is intact"},
		},
		{
			name:       "agent.json malformed",
			tamperCmd:  fmt.Sprintf("printf '{' > %s", agentPath),
			wantOutput: []string{"malformed evidence agent.json", "parse agent.json", "evidence is intact"},
		},
		{
			name:       "agent.json parses but is incomplete",
			tamperCmd:  fmt.Sprintf(`printf '{"x":1}' > %s`, agentPath),
			wantOutput: []string{"malformed evidence agent.json", "schema_version", "evidence is intact"},
		},
	}

	// Subtests run sequentially: each mutates the shared evidence directory and
	// restores it in cleanup, so parallel execution would race.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execCmd(t, env, ctx, tt.tamperCmd, "tamper evidence")
			t.Cleanup(func() {
				execCmd(t, env, ctx, restoreCmd, "restore evidence")
			})

			promoteCode, promoteOutput := runPromote(t, env, runID, "")
			require.NotEqual(t, 0, promoteCode, "promote must fail: %s", promoteOutput)
			for _, want := range tt.wantOutput {
				require.Contains(t, promoteOutput, want)
			}

			requireGitStateUnchanged(t, env, repoPath, baseline)
		})
	}

	// Control: with the evidence restored the same run promotes cleanly, so the
	// rejections above are attributable to the tampering rather than to an
	// unrelated defect in the fixture.
	promoteCode, promoteOutput := runPromote(t, env, runID, "")
	require.Equal(t, 0, promoteCode, "promote should succeed once evidence is restored: %s", promoteOutput)
	require.Contains(t, promoteOutput, "branch: tessariq/"+runID)
}

// gitState is a snapshot of the repository used to prove that a rejected
// promote produced no Git side effects: no new branch, no new commit, and no
// working-tree changes.
//
// refs is the load-bearing field for evidence-tamper tests, since promote's
// only side effect on repoRoot is creating refs/heads/tessariq/<run_id>. status
// cannot observe evidence tampering — `tessariq init` gitignores `.tessariq/`,
// so mutations under the evidence directory never reach `git status`; it guards
// the separate case of promote touching tracked files.
type gitState struct {
	head   string
	refs   string
	status string
}

// captureCleanBaseline snapshots the repository immediately before the promote
// under test and asserts up front that the working tree is already clean. The
// pre-promote assertion is what makes a later failure attributable: without it,
// a setup regression that left the repo dirty would surface as the post-promote
// "must stay clean" failure and point at the wrong step.
func captureCleanBaseline(t *testing.T, env *containers.RunEnv, repoPath string) gitState {
	t.Helper()

	baseline := captureGitState(t, env, repoPath)
	require.Empty(t, baseline.status, "working tree must be clean before promote")
	return baseline
}

func captureGitState(t *testing.T, env *containers.RunEnv, repoPath string) gitState {
	t.Helper()

	return gitState{
		head:   gitOutput(t, env, repoPath, "rev-parse HEAD"),
		refs:   gitOutput(t, env, repoPath, "for-each-ref --format='%(refname)' refs/heads/"),
		status: gitOutput(t, env, repoPath, "status --porcelain"),
	}
}

// gitOutput runs a git command against repoPath inside the e2e container and
// returns its trimmed stdout. args is a shell fragment, so pipelines are allowed.
func gitOutput(t *testing.T, env *containers.RunEnv, repoPath, args string) string {
	t.Helper()

	cmd := fmt.Sprintf("git -C %s %s", repoPath, args)
	code, out, err := env.Exec(context.Background(), []string{"sh", "-c", cmd})
	require.NoError(t, err, "git %s: %s", args, out)
	require.Equal(t, 0, code, "git %s exited %d: %s", args, code, out)
	return strings.TrimSpace(out)
}

func requireGitStateUnchanged(t *testing.T, env *containers.RunEnv, repoPath string, baseline gitState) {
	t.Helper()

	after := captureGitState(t, env, repoPath)
	require.Equal(t, baseline.head, after.head, "HEAD must not move on a rejected promote")
	require.Equal(t, baseline.refs, after.refs, "no branch may be created on a rejected promote")
	require.Empty(t, after.status, "working tree must stay clean on a rejected promote")
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
