//go:build integration

package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestProvision_Integration_CreatesWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	wsPath, baseSHA, err := Provision(ctx, homeDir, repo.Dir(), runID, evidenceDir)
	require.NoError(t, err)

	// Worktree directory exists
	info, err := os.Stat(wsPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	// Base SHA is a valid 40-char hex
	require.Len(t, baseSHA, 40)
	require.Regexp(t, `^[0-9a-f]{40}$`, baseSHA)
}

func TestProvision_Integration_WritesWorkspaceJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	wsPath, baseSHA, err := Provision(ctx, homeDir, repo.Dir(), runID, evidenceDir)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(evidenceDir, "workspace.json"))
	require.NoError(t, err)

	var m Metadata
	require.NoError(t, json.Unmarshal(data, &m))

	require.Equal(t, 1, m.SchemaVersion)
	require.Equal(t, "worktree", m.WorkspaceMode)
	require.Equal(t, baseSHA, m.BaseSHA)
	require.Equal(t, wsPath, m.WorkspacePath)
	require.Equal(t, "rw", m.RepoMountMode)
	require.True(t, m.RepoClean)
	require.Equal(t, "strong", m.Reproducibility)
}

func TestProvision_Integration_MatchesContainerSHA(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	expected, err := repo.ExecOutput(ctx, "rev-parse", "HEAD")
	require.NoError(t, err)

	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	_, baseSHA, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir)
	require.NoError(t, err)
	require.Equal(t, expected, baseSHA)
}

func TestCleanup_Integration_RemovesWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, _, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir)
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

	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, _, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir)
	require.NoError(t, err)

	require.NoError(t, Cleanup(ctx, repo.Dir(), wsPath))
	require.NoError(t, Cleanup(ctx, repo.Dir(), wsPath))
}
