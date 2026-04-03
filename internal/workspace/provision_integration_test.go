//go:build integration

package workspace

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

func TestWorkspacePath_Layout(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	repoRoot := "/home/user/code/tessariq"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	p := WorkspacePath(homeDir, repoRoot, runID)

	require.Contains(t, p, filepath.Join(homeDir, ".tessariq", "worktrees"))
	require.Contains(t, p, runID)
	require.Contains(t, p, RepoID(repoRoot))
}

func resolveHead(t *testing.T, ctx context.Context, repo *containers.GitRepo) string {
	t.Helper()
	sha, err := repo.ExecOutput(ctx, "rev-parse", "HEAD")
	require.NoError(t, err)
	return sha
}

func TestProvision_Integration_CreatesWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), runID, evidenceDir, headSHA)
	require.NoError(t, err)

	// Worktree directory exists
	info, err := os.Stat(wsPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestProvision_Integration_WritesWorkspaceJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), runID, evidenceDir, headSHA)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(evidenceDir, "workspace.json"))
	require.NoError(t, err)

	var m Metadata
	require.NoError(t, json.Unmarshal(data, &m))

	require.Equal(t, 1, m.SchemaVersion)
	require.Equal(t, "worktree", m.WorkspaceMode)
	require.Equal(t, headSHA, m.BaseSHA)
	require.Equal(t, wsPath, m.WorkspacePath)
	require.Equal(t, "rw", m.RepoMountMode)
	require.True(t, m.RepoClean)
	require.Equal(t, "strong", m.Reproducibility)
}

func TestProvision_Integration_WorktreeCheckedOutAtProvidedSHA(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir, headSHA)
	require.NoError(t, err)

	// Verify the worktree is checked out at the provided SHA.
	out, err := exec.CommandContext(ctx, "git", "-C", wsPath, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	require.Equal(t, headSHA, strings.TrimSpace(string(out)))
}

func TestProvision_Integration_UsesCallerProvidedSHA(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	// Capture the first commit SHA.
	firstSHA := resolveHead(t, ctx, repo)

	// Advance HEAD with a second commit so HEAD != firstSHA.
	_, err = repo.ExecOutput(ctx, "commit", "--allow-empty", "-m", "second commit")
	require.NoError(t, err)
	secondSHA := resolveHead(t, ctx, repo)
	require.NotEqual(t, firstSHA, secondSHA, "HEAD must have advanced")

	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	// Pass the OLD SHA — Provision must use it, not current HEAD.
	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir, firstSHA)
	require.NoError(t, err)

	// workspace.json must record the caller-provided SHA.
	data, err := os.ReadFile(filepath.Join(evidenceDir, "workspace.json"))
	require.NoError(t, err)

	var m Metadata
	require.NoError(t, json.Unmarshal(data, &m))
	require.Equal(t, firstSHA, m.BaseSHA)

	// The worktree must be checked out at the caller-provided SHA.
	out, err := exec.CommandContext(ctx, "git", "-C", wsPath, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	require.Equal(t, firstSHA, strings.TrimSpace(string(out)))
}

func TestCleanup_Integration_RemovesWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir, headSHA)
	require.NoError(t, err)

	require.NoError(t, Cleanup(ctx, repo.Dir(), wsPath))

	_, err = os.Stat(wsPath)
	require.True(t, os.IsNotExist(err))
}

func TestCleanup_Integration_Idempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir, headSHA)
	require.NoError(t, err)

	require.NoError(t, Cleanup(ctx, repo.Dir(), wsPath))
	require.NoError(t, Cleanup(ctx, repo.Dir(), wsPath))
}

func TestCleanup_Integration_GitWorktreeListClean(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAX", evidenceDir, headSHA)
	require.NoError(t, err)

	require.NoError(t, Cleanup(ctx, repo.Dir(), wsPath))

	// git worktree list must show only the main worktree after cleanup.
	out, err := exec.CommandContext(ctx, "git", "-C", repo.Dir(), "worktree", "list", "--porcelain").CombinedOutput()
	require.NoError(t, err, "git worktree list: %s", out)

	worktreeCount := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			worktreeCount++
		}
	}
	require.Equal(t, 1, worktreeCount, "only the main worktree should remain after cleanup, got: %s", out)
}

func TestCleanup_Integration_RemovesWorktreeRef_WhenRepairFails(t *testing.T) {
	// No t.Parallel() — t.Setenv is not compatible with parallel tests.

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAR", evidenceDir, headSHA)
	require.NoError(t, err)

	// Simulate Docker unavailability by prepending a directory with a fake
	// docker binary that always fails, so repairWorkspaceOwnership errors out
	// while git remains available.
	fakeDockerDir := t.TempDir()
	fakePath := filepath.Join(fakeDockerDir, "docker")
	require.NoError(t, os.WriteFile(fakePath, []byte("#!/bin/sh\nexit 1\n"), 0o755))
	t.Setenv("PATH", fakeDockerDir+":"+os.Getenv("PATH"))

	err = Cleanup(ctx, repo.Dir(), wsPath)
	require.NoError(t, err, "cleanup must succeed even when Docker repair fails")

	// Verify the workspace directory is gone.
	_, statErr := os.Stat(wsPath)
	require.True(t, os.IsNotExist(statErr), "workspace directory must be removed")

	// Verify git worktree ref is cleaned up.
	out, err := exec.CommandContext(ctx, "git", "-C", repo.Dir(), "worktree", "list", "--porcelain").CombinedOutput()
	require.NoError(t, err, "git worktree list: %s", out)

	worktreeCount := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			worktreeCount++
		}
	}
	require.Equal(t, 1, worktreeCount, "only main worktree should remain, got: %s", out)
}

func TestCleanup_Integration_RemovesRestrictiveContainerOwnedFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAW", evidenceDir, headSHA)
	require.NoError(t, err)

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-v", wsPath+":/work", "alpine:latest",
		"sh", "-c", "mkdir -p /work/private && touch /work/private/file && chmod 700 /work/private")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "create restrictive files: %s", out)

	require.NoError(t, Cleanup(ctx, repo.Dir(), wsPath))

	_, err = os.Stat(wsPath)
	require.True(t, os.IsNotExist(err))
}
