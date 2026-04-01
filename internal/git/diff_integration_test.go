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

func TestDiff_Integration_NoChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	baseSHA, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)

	out, err := Diff(ctx, repo.Dir(), baseSHA)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestDiff_Integration_WithChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	baseSHA, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)

	// Create a new file in the worktree.
	require.NoError(t, os.WriteFile(filepath.Join(repo.Dir(), "newfile.txt"), []byte("hello\n"), 0o644))

	out, err := Diff(ctx, repo.Dir(), baseSHA)
	require.NoError(t, err)
	require.NotEmpty(t, out)
	require.Contains(t, string(out), "newfile.txt")
	require.Contains(t, string(out), "+hello")
}

func TestDiffStat_Integration_NoChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	baseSHA, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)

	out, err := DiffStat(ctx, repo.Dir(), baseSHA)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestDiffStat_Integration_WithChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	baseSHA, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(repo.Dir(), "newfile.txt"), []byte("hello\n"), 0o644))

	out, err := DiffStat(ctx, repo.Dir(), baseSHA)
	require.NoError(t, err)
	require.NotEmpty(t, out)
	require.Contains(t, string(out), "newfile.txt")
}
