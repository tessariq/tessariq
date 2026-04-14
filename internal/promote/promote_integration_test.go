//go:build integration

package promote

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/adapter"
	"github.com/tessariq/tessariq/internal/proxy"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/testutil/containers"
	"github.com/tessariq/tessariq/internal/workspace"
)

const testRunID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestRun_CreatesBranchAndSingleCommit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	writeFile(t, filepath.Join(repo.Dir(), "deleted.txt"), "remove me\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt", "deleted.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
		require.NoError(t, os.Remove(filepath.Join(worktree, "deleted.txt")))
	})

	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)

	result, err := Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.NoError(t, err)
	require.Equal(t, testRunID, result.RunID)
	require.Equal(t, defaultBranchName(testRunID), result.Branch)

	branchSHA := gitOutputTest(t, repo.Dir(), "rev-parse", result.Branch)
	require.Equal(t, result.Commit, branchSHA)
	require.Equal(t, baseSHA, gitOutputTest(t, repo.Dir(), "rev-parse", result.Branch+"^"))

	body := gitOutputTest(t, repo.Dir(), "log", "-1", "--format=%B", result.Branch)
	require.Contains(t, body, "Promote Sample Task")
	require.Contains(t, body, "Tessariq-Run: "+testRunID)
	require.Contains(t, body, "Tessariq-Base: "+baseSHA)
	require.Contains(t, body, "Tessariq-Task: tasks/sample.md")

	show := gitOutputTest(t, repo.Dir(), "show", "--stat", "--format=", result.Branch)
	require.Contains(t, show, "tracked.txt")
	require.Contains(t, show, "deleted.txt")
	require.Contains(t, show, "2 files changed")
}

func TestRun_PromotesBinaryFileChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")

	// Binary content with null bytes so git treats it as binary.
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE}

	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		require.NoError(t, os.WriteFile(filepath.Join(worktree, "image.png"), binaryContent, 0o644))
	})
	require.Contains(t, patch, "GIT binary patch", "patch must contain binary data")

	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)

	result, err := Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.NoError(t, err)

	// Read the binary file from the promoted branch and verify byte-for-byte equality.
	promoted, err := exec.CommandContext(ctx, "git", "-C", repo.Dir(), "show", result.Branch+":image.png").Output()
	require.NoError(t, err)
	require.Equal(t, binaryContent, promoted, "promoted branch must contain exact binary bytes")
}

func TestRun_ZeroDiffFailsWithoutBranchOrCommit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, "", "")

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.ErrorIs(t, err, ErrNoCodeChanges)
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", defaultBranchName(testRunID)))
	branchCount := gitOutputTest(t, repo.Dir(), "rev-list", "--count", "--all")
	require.Equal(t, "2", branchCount)
}

func TestRun_SecondPromoteFailsCleanly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.NoError(t, err)

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.ErrorIs(t, err, ErrBranchExists)
	require.Contains(t, err.Error(), defaultBranchName(testRunID))
}

func TestRun_CustomMessageWithoutTrailers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)

	message := "Line one ✨\n\nLine two"
	result, err := Run(ctx, repo.Dir(), Options{
		RunRef:     testRunID,
		Branch:     "feature/custom-message",
		Message:    message,
		NoTrailers: true,
	})
	require.NoError(t, err)

	body := gitOutputRawTest(t, repo.Dir(), "log", "-1", "--format=%B", result.Branch)
	require.Equal(t, message, strings.TrimRight(body, "\n"))
	require.NotContains(t, body, "Tessariq-Run:")
}

func TestRun_InvalidBranchNameFailsCleanly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID, Branch: "bad name"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid branch name")
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", "bad name"))
}

func TestRun_MissingEvidenceIdentifiesArtifact(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)
	require.NoError(t, os.Remove(filepath.Join(repo.Dir(), ".tessariq", "runs", testRunID, "workspace.json")))

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "workspace.json")
	require.Contains(t, err.Error(), "evidence is intact")
}

func TestRun_MissingDiffstatRejectsPromote(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, _ := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	// Create evidence with diff.patch but NO diffstat.txt.
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, "")

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "diffstat.txt")
	require.Contains(t, err.Error(), "evidence is intact")
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", defaultBranchName(testRunID)))
}

func TestRun_EmptyDiffstatRejectsPromote(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, _ := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	// Create evidence with diff.patch and an empty diffstat.txt.
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, "")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo.Dir(), ".tessariq", "runs", testRunID, "diffstat.txt"),
		[]byte{}, 0o600,
	))

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "diffstat.txt")
	require.Contains(t, err.Error(), "evidence is intact")
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", defaultBranchName(testRunID)))
}

func TestRun_ProxyRunWithBothEgressArtifactsPromotes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)
	markProxyEgressMode(t, repo.Dir(), testRunID)
	writeProxyEgressArtifacts(t, repo.Dir(), testRunID)

	result, err := Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.NoError(t, err)
	require.Equal(t, testRunID, result.RunID)
}

func TestRun_ProxyRunMissingCompiledYAMLRejectsPromote(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)
	markProxyEgressMode(t, repo.Dir(), testRunID)
	writeProxyEgressArtifacts(t, repo.Dir(), testRunID)
	require.NoError(t, os.Remove(filepath.Join(repo.Dir(), ".tessariq", "runs", testRunID, "egress.compiled.yaml")))

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "egress.compiled.yaml")
	require.Contains(t, err.Error(), "evidence is intact")
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", defaultBranchName(testRunID)))
}

func TestRun_ProxyRunMissingEventsJSONLRejectsPromote(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)
	markProxyEgressMode(t, repo.Dir(), testRunID)
	writeProxyEgressArtifacts(t, repo.Dir(), testRunID)
	require.NoError(t, os.WriteFile(
		filepath.Join(repo.Dir(), ".tessariq", "runs", testRunID, "egress.events.jsonl"),
		[]byte{}, 0o600,
	))

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "egress.events.jsonl")
	require.Contains(t, err.Error(), "evidence is intact")
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", defaultBranchName(testRunID)))
}

func TestRun_IncompleteIndexFailsCleanly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	// Create a valid evidence fixture so the only issue is the index shape.
	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	evidenceDir := filepath.Join(repo.Dir(), ".tessariq", "runs", testRunID)
	require.NoError(t, os.MkdirAll(evidenceDir, 0o700))
	require.NoError(t, run.WriteManifest(evidenceDir, run.Manifest{
		SchemaVersion: 1,
		RunID:         testRunID,
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "Sample Task",
		Agent:         "claude-code",
		BaseSHA:       baseSHA,
		WorkspaceMode: "worktree",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}))
	require.NoError(t, runner.WriteStatus(evidenceDir, runner.NewTerminalStatus(runner.StateSuccess, time.Now().Add(-time.Minute), time.Now(), 0, false)))
	require.NoError(t, adapter.WriteAgentInfo(evidenceDir, adapter.NewAgentInfo("claude-code", map[string]any{}, map[string]bool{})))
	require.NoError(t, adapter.WriteRuntimeInfo(evidenceDir, adapter.NewRuntimeInfo("test-image", "custom", 0, "disabled", "disabled")))
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata(baseSHA, "/tmp/worktree")))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "task.md"), []byte("# Sample Task\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "run.log"), []byte("ok\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "runner.log"), []byte("ok\n"), 0o600))
	if patch != "" {
		require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "diff.patch"), []byte(patch), 0o600))
	}
	if diffstat != "" {
		require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "diffstat.txt"), []byte(diffstat), 0o600))
	}

	// Write only incomplete index entries (missing required fields).
	runsDir := filepath.Join(repo.Dir(), ".tessariq", "runs")
	require.NoError(t, os.MkdirAll(runsDir, 0o700))
	incompleteIndex := `{"run_id":"` + testRunID + `","state":"success"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(runsDir, "index.jsonl"), []byte(incompleteIndex), 0o600))

	_, err = Run(ctx, repo.Dir(), Options{RunRef: "last"})
	require.ErrorIs(t, err, run.ErrEmptyIndex)
}

func TestRun_ForgedExternalEvidencePathRejectedBeforeGitSideEffects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	// Write a forged index entry with an absolute external evidence path.
	runsDir := filepath.Join(repo.Dir(), ".tessariq", "runs")
	require.NoError(t, os.MkdirAll(runsDir, 0o700))
	forgedEntry := run.IndexEntry{
		RunID:         testRunID,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "Forged Task",
		Agent:         "claude-code",
		WorkspaceMode: "worktree",
		State:         string(runner.StateSuccess),
		EvidencePath:  "/tmp/evil-evidence",
	}
	require.NoError(t, run.AppendIndex(runsDir, forgedEntry))

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.Error(t, err)
	require.ErrorIs(t, err, run.ErrEvidencePathOutsideRepo)
	require.Contains(t, err.Error(), "outside the repository")

	// Assert no branch was created (no git side effects).
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", defaultBranchName(testRunID)))
	branchCount := gitOutputTest(t, repo.Dir(), "rev-list", "--count", "--all")
	require.Equal(t, "2", branchCount, "expected only the initial 2 commits, no promote commit")
}

func TestRun_ForgedTraversalEvidencePathRejectedBeforeGitSideEffects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	// Write a forged index entry with a path-traversal evidence path.
	runsDir := filepath.Join(repo.Dir(), ".tessariq", "runs")
	require.NoError(t, os.MkdirAll(runsDir, 0o700))
	forgedEntry := run.IndexEntry{
		RunID:         testRunID,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "Forged Task",
		Agent:         "claude-code",
		WorkspaceMode: "worktree",
		State:         string(runner.StateSuccess),
		EvidencePath:  ".tessariq/runs/../../etc/passwd",
	}
	require.NoError(t, run.AppendIndex(runsDir, forgedEntry))

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.Error(t, err)
	require.ErrorIs(t, err, run.ErrEvidencePathOutsideRepo)

	// Assert no branch was created.
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", defaultBranchName(testRunID)))
}

func TestRun_TamperedManifestRunIDRejectedBeforeGitSideEffects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	writeFile(t, filepath.Join(repo.Dir(), "tracked.txt"), "before\n")
	gitRunTest(t, repo.Dir(), "add", "tracked.txt")
	gitRunTest(t, repo.Dir(), "commit", "-m", "base")

	baseSHA := gitOutputTest(t, repo.Dir(), "rev-parse", "HEAD")
	patch, diffstat := buildDiffArtifacts(t, repo.Dir(), baseSHA, func(worktree string) {
		writeFile(t, filepath.Join(worktree, "tracked.txt"), "after\n")
	})
	createEvidenceFixture(t, repo.Dir(), testRunID, baseSHA, patch, diffstat)

	// Tamper with manifest.json: overwrite run_id with a different value.
	tamperedRunID := "01BBBBBBBBBBBBBBBBBBBBBBBBB"
	evidenceDir := filepath.Join(repo.Dir(), ".tessariq", "runs", testRunID)
	require.NoError(t, run.WriteManifest(evidenceDir, run.Manifest{
		SchemaVersion: 1,
		RunID:         tamperedRunID,
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "Tampered Task",
		Agent:         "claude-code",
		BaseSHA:       baseSHA,
		WorkspaceMode: "worktree",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}))

	_, err = Run(ctx, repo.Dir(), Options{RunRef: testRunID})
	require.ErrorIs(t, err, ErrManifestIdentityMismatch)
	require.Contains(t, err.Error(), tamperedRunID)
	require.Contains(t, err.Error(), "tampered")

	// Assert no branch was created (no git side effects).
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", defaultBranchName(testRunID)))
	require.Empty(t, gitOutputAllowFailure(t, repo.Dir(), "branch", "--list", defaultBranchName(tamperedRunID)))
	branchCount := gitOutputTest(t, repo.Dir(), "rev-list", "--count", "--all")
	require.Equal(t, "2", branchCount, "expected only the initial 2 commits, no promote commit")
}

func buildDiffArtifacts(t *testing.T, repoDir, baseSHA string, mutate func(string)) (string, string) {
	t.Helper()

	parent := t.TempDir()
	worktree := filepath.Join(parent, "worktree")
	gitRunTest(t, repoDir, "worktree", "add", "--detach", worktree, baseSHA)
	defer gitRunTest(t, repoDir, "worktree", "remove", "--force", worktree)

	mutate(worktree)
	gitRunTest(t, worktree, "add", "-N", ".")

	patch := gitOutputRawTest(t, worktree, "diff", "--binary", baseSHA, "--", ".")
	diffstat := gitOutputRawTest(t, worktree, "diff", "--stat", baseSHA, "--", ".")
	return patch, diffstat
}

func createEvidenceFixture(t *testing.T, repoDir, runID, baseSHA, patch, diffstat string) {
	t.Helper()

	evidenceDir := filepath.Join(repoDir, ".tessariq", "runs", runID)
	require.NoError(t, os.MkdirAll(evidenceDir, 0o700))

	manifest := run.Manifest{
		SchemaVersion: 1,
		RunID:         runID,
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "Promote Sample Task",
		Agent:         "claude-code",
		BaseSHA:       baseSHA,
		WorkspaceMode: "worktree",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, run.WriteManifest(evidenceDir, manifest))
	require.NoError(t, runner.WriteStatus(evidenceDir, runner.NewTerminalStatus(runner.StateSuccess, time.Now().Add(-time.Minute), time.Now(), 0, false)))
	require.NoError(t, adapter.WriteAgentInfo(evidenceDir, adapter.NewAgentInfo("claude-code", map[string]any{}, map[string]bool{})))
	require.NoError(t, adapter.WriteRuntimeInfo(evidenceDir, adapter.NewRuntimeInfo("test-image", "custom", 0, "disabled", "disabled")))
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata(baseSHA, "/tmp/cleaned-worktree")))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "task.md"), []byte("# Promote Sample Task\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "run.log"), []byte("ok\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "runner.log"), []byte("ok\n"), 0o600))
	if patch != "" {
		require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "diff.patch"), []byte(patch), 0o600))
	}
	if diffstat != "" {
		require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "diffstat.txt"), []byte(diffstat), 0o600))
	}

	entry := run.IndexEntry{
		RunID:         runID,
		CreatedAt:     manifest.CreatedAt,
		TaskPath:      manifest.TaskPath,
		TaskTitle:     manifest.TaskTitle,
		Agent:         manifest.Agent,
		WorkspaceMode: manifest.WorkspaceMode,
		State:         string(runner.StateSuccess),
		EvidencePath:  filepath.Join(".tessariq", "runs", runID),
	}
	require.NoError(t, run.AppendIndex(filepath.Join(repoDir, ".tessariq", "runs"), entry))
}

func markProxyEgressMode(t *testing.T, repoDir, runID string) {
	t.Helper()
	evidenceDir := filepath.Join(repoDir, ".tessariq", "runs", runID)
	m, err := run.ReadManifest(evidenceDir)
	require.NoError(t, err)
	m.ResolvedEgressMode = "proxy"
	m.RequestedEgressMode = "proxy"
	require.NoError(t, run.WriteManifest(evidenceDir, m))
}

func writeProxyEgressArtifacts(t *testing.T, repoDir, runID string) {
	t.Helper()
	evidenceDir := filepath.Join(repoDir, ".tessariq", "runs", runID)
	compiled, err := proxy.NewCompiledAllowlist("custom", []string{"api.example.com:443"})
	require.NoError(t, err)
	require.NoError(t, proxy.WriteCompiledYAML(evidenceDir, compiled))
	require.NoError(t, proxy.WriteEventsJSONL(evidenceDir, []proxy.Event{
		{
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			Host:        "blocked.example.com",
			Port:        443,
			Action:      "blocked",
			Reason:      "not_in_allowlist",
			SquidResult: "TCP_DENIED/403",
		},
	}))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func gitRunTest(t *testing.T, repoDir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", repoDir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

func gitOutputTest(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", repoDir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func gitOutputAllowFailure(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", repoDir}, args...)...)
	out, _ := cmd.CombinedOutput()
	return strings.TrimSpace(string(out))
}

func gitOutputRawTest(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", repoDir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return string(out)
}
