//go:build integration

package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

func TestAddWorktree_Integration_CreatesDirectory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	sha, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)

	wtPath := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, AddWorktree(ctx, repo.Dir(), wtPath, sha))

	info, err := os.Stat(wtPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestAddWorktree_Integration_DetachedHead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	sha, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)

	wtPath := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, AddWorktree(ctx, repo.Dir(), wtPath, sha))

	// Verify detached HEAD by checking that symbolic-ref fails
	headSHA, err := HeadSHA(ctx, wtPath)
	require.NoError(t, err)
	require.Equal(t, sha, headSHA)
}

func TestAddWorktree_Integration_ChecksOutCorrectCommit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	// Create a second commit
	require.NoError(t, repo.Exec(ctx, "commit", "--allow-empty", "-m", "second"))
	secondSHA, err := repo.ExecOutput(ctx, "rev-parse", "HEAD")
	require.NoError(t, err)

	// Get the first commit
	firstSHA, err := repo.ExecOutput(ctx, "rev-parse", "HEAD~1")
	require.NoError(t, err)

	// Create worktree at the first commit
	wtPath := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, AddWorktree(ctx, repo.Dir(), wtPath, firstSHA))

	// Verify the worktree HEAD is at the first commit, not the second
	headSHA, err := HeadSHA(ctx, wtPath)
	require.NoError(t, err)
	require.Equal(t, firstSHA, headSHA)
	require.NotEqual(t, secondSHA, headSHA)
}

func TestRemoveWorktree_Integration_RemovesWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	sha, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)

	wtPath := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, AddWorktree(ctx, repo.Dir(), wtPath, sha))
	require.NoError(t, RemoveWorktree(ctx, repo.Dir(), wtPath))

	_, err = os.Stat(wtPath)
	require.True(t, os.IsNotExist(err))
}

func TestRemoveWorktree_Integration_DirtyWorktreeWithForce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	sha, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)

	wtPath := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, AddWorktree(ctx, repo.Dir(), wtPath, sha))

	// Make the worktree dirty
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("dirty"), 0o644))

	// Should succeed with --force
	require.NoError(t, RemoveWorktree(ctx, repo.Dir(), wtPath))
}
