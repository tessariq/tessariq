//go:build integration

package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

func TestHeadSHA_Integration_ReturnsValidSHA(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	sha, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)
	require.Len(t, sha, 40, "SHA must be 40 hex characters")
	require.Regexp(t, `^[0-9a-f]{40}$`, sha)
}

func TestHeadSHA_Integration_MatchesContainerSHA(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	expected, err := repo.ExecOutput(ctx, "rev-parse", "HEAD")
	require.NoError(t, err)

	sha, err := HeadSHA(ctx, repo.Dir())
	require.NoError(t, err)
	require.Equal(t, expected, sha)
}

func TestHeadSHA_Integration_FailsOnNonGitDir(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()

	_, err := HeadSHA(ctx, dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "resolve HEAD")
}
