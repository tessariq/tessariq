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

func TestDiff_Integration_WithBinaryChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	baseSHA, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)

	// Write a file containing null bytes so git treats it as binary.
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x01, 0x02, 0x03}
	require.NoError(t, os.WriteFile(filepath.Join(repo.Dir(), "image.png"), binaryContent, 0o644))

	out, err := Diff(ctx, repo.Dir(), baseSHA)
	require.NoError(t, err)
	require.NotEmpty(t, out)
	require.Contains(t, string(out), "image.png")
	require.Contains(t, string(out), "GIT binary patch", "diff must include binary patch data, not just a placeholder")
	require.NotContains(t, string(out), "Binary files", "diff must not fall back to text-only placeholder")
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
