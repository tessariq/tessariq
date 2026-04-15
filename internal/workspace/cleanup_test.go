package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanup_NonExistentPath_ReturnsNil(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	// A non-existent canonical workspace path under the proper tree.
	wsPath := WorkspacePath(homeDir, "/tmp/fake-repo-root", "01ARZ3NDEKTSV4RRFFQ69G5FAV")

	err := Cleanup(context.Background(), homeDir, "/tmp/fake-repo-root", wsPath)
	require.NoError(t, err, "Cleanup must return nil for a non-existent canonical workspace path")
}

func TestCleanup_RemovesDirectory_WhenRepairFails(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	repoRoot := "/tmp/fake-repo-root"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	// Provision a canonical workspace directory so the guard accepts it.
	wsPath := WorkspacePath(homeDir, repoRoot, runID)
	require.NoError(t, os.MkdirAll(filepath.Join(wsPath, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wsPath, "subdir", "file.txt"), []byte("data"), 0o644))

	err := Cleanup(context.Background(), homeDir, repoRoot, wsPath)
	require.NoError(t, err)

	_, statErr := os.Stat(wsPath)
	require.True(t, os.IsNotExist(statErr), "workspace directory must be removed even when repair fails")
}

func TestCleanup_Idempotent_AfterRepairFailure(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	repoRoot := "/tmp/fake-repo-root"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	wsPath := WorkspacePath(homeDir, repoRoot, runID)
	require.NoError(t, os.MkdirAll(wsPath, 0o755))

	// First call: repair fails, but directory is removed.
	require.NoError(t, Cleanup(context.Background(), homeDir, repoRoot, wsPath))
	// Second call: path is already gone — must return nil.
	require.NoError(t, Cleanup(context.Background(), homeDir, repoRoot, wsPath))
}

// TestCleanup_RejectsPathOutsideWorktreesTree is the defensive safety net.
// Even if a future caller forgets to validate first, Cleanup must refuse to
// chown, chmod, or os.RemoveAll a path that is not contained under
// <homeDir>/.tessariq/worktrees/.
func TestCleanup_RejectsPathOutsideWorktreesTree(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()

	// Decoy lives completely outside the worktrees tree.
	decoy := filepath.Join(t.TempDir(), "decoy")
	sentinel := filepath.Join(decoy, "sentinel.txt")
	require.NoError(t, os.MkdirAll(decoy, 0o755))
	require.NoError(t, os.WriteFile(sentinel, []byte("do-not-touch"), 0o600))

	err := Cleanup(context.Background(), homeDir, "/fake/repo", decoy)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrWorkspacePathOutsideTree)

	info, statErr := os.Stat(decoy)
	require.NoError(t, statErr, "decoy directory must not be touched")
	require.True(t, info.IsDir())
	data, readErr := os.ReadFile(sentinel)
	require.NoError(t, readErr)
	require.Equal(t, "do-not-touch", string(data))
}

func TestCleanup_RejectsRelativePath(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	err := Cleanup(context.Background(), homeDir, "/fake/repo", "relative/path")
	require.ErrorIs(t, err, ErrWorkspacePathOutsideTree)
}
