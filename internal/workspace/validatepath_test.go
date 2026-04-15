package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testRunID       = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	otherTestRunID  = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	testRepoFixture = "/fixtures/tessariq"
)

func mkCanonical(t *testing.T, homeDir, repoRoot, runID string) string {
	t.Helper()
	p := WorkspacePath(homeDir, repoRoot, runID)
	require.NoError(t, os.MkdirAll(p, 0o755))
	return p
}

func TestValidateWorkspacePath_HappyPath(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	canonical := mkCanonical(t, homeDir, testRepoFixture, testRunID)

	real, err := ValidateWorkspacePath(homeDir, testRepoFixture, testRunID, canonical)
	require.NoError(t, err)

	wantReal, err := filepath.EvalSymlinks(canonical)
	require.NoError(t, err)
	require.Equal(t, wantReal, real)
}

func TestValidateWorkspacePath_EmptyIsNoop(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	real, err := ValidateWorkspacePath(homeDir, testRepoFixture, testRunID, "")
	require.NoError(t, err)
	require.Equal(t, "", real)
}

func TestValidateWorkspacePath_RelativePathRejected(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	_, err := ValidateWorkspacePath(homeDir, testRepoFixture, testRunID, "relative/path")
	require.ErrorIs(t, err, ErrWorkspacePathOutsideTree)
}

func TestValidateWorkspacePath_OutsideWorktreesTreeRejected(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	// Make sure the canonical path exists so the failure is not "missing canonical".
	mkCanonical(t, homeDir, testRepoFixture, testRunID)

	// decoy under a separate directory entirely.
	decoy := filepath.Join(t.TempDir(), "evil")
	require.NoError(t, os.MkdirAll(decoy, 0o755))

	_, err := ValidateWorkspacePath(homeDir, testRepoFixture, testRunID, decoy)
	require.ErrorIs(t, err, ErrWorkspacePathOutsideTree)
}

func TestValidateWorkspacePath_WrongRunIDRejected(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	mkCanonical(t, homeDir, testRepoFixture, testRunID)
	// A legitimate-looking worktree for a *different* run.
	otherCanonical := mkCanonical(t, homeDir, testRepoFixture, otherTestRunID)

	_, err := ValidateWorkspacePath(homeDir, testRepoFixture, testRunID, otherCanonical)
	require.ErrorIs(t, err, ErrWorkspacePathOutsideTree)
}

func TestValidateWorkspacePath_WrongRepoIDRejected(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	mkCanonical(t, homeDir, testRepoFixture, testRunID)
	// Worktree under a DIFFERENT repo's id but the same runID — still inside
	// ~/.tessariq/worktrees/ but not this repo's canonical path.
	other := mkCanonical(t, homeDir, "/fixtures/other-repo", testRunID)

	_, err := ValidateWorkspacePath(homeDir, testRepoFixture, testRunID, other)
	require.ErrorIs(t, err, ErrWorkspacePathOutsideTree)
}

func TestValidateWorkspacePath_SymlinkLeafEscape(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	// Build the parent tree but skip creating the canonical leaf.
	canonical := WorkspacePath(homeDir, testRepoFixture, testRunID)
	require.NoError(t, os.MkdirAll(filepath.Dir(canonical), 0o755))

	// Decoy lives outside the worktrees tree.
	decoy := filepath.Join(t.TempDir(), "evil")
	require.NoError(t, os.MkdirAll(decoy, 0o755))

	// Plant a symlink at the canonical leaf pointing at the decoy.
	require.NoError(t, os.Symlink(decoy, canonical))

	_, err := ValidateWorkspacePath(homeDir, testRepoFixture, testRunID, canonical)
	require.ErrorIs(t, err, ErrWorkspacePathOutsideTree)
}

func TestValidateWorkspacePath_SymlinkAncestorEscape(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()

	// Plant a symlink at ~/.tessariq/worktrees pointing at an external
	// directory that then contains a matching repoID/runID leaf.
	worktreesDir := filepath.Join(homeDir, ".tessariq", "worktrees")
	require.NoError(t, os.MkdirAll(filepath.Dir(worktreesDir), 0o755))

	external := filepath.Join(t.TempDir(), "external")
	leaf := filepath.Join(external, RepoID(testRepoFixture), testRunID)
	require.NoError(t, os.MkdirAll(leaf, 0o755))

	require.NoError(t, os.Symlink(external, worktreesDir))

	canonical := WorkspacePath(homeDir, testRepoFixture, testRunID)
	_, err := ValidateWorkspacePath(homeDir, testRepoFixture, testRunID, canonical)
	require.ErrorIs(t, err, ErrWorkspacePathOutsideTree)
}

func TestValidateWorkspacePath_NonexistentCanonicalRejected(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	canonical := WorkspacePath(homeDir, testRepoFixture, testRunID)
	// Do not create the canonical path.

	_, err := ValidateWorkspacePath(homeDir, testRepoFixture, testRunID, canonical)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrWorkspacePathOutsideTree) || errors.Is(err, os.ErrNotExist))
}
