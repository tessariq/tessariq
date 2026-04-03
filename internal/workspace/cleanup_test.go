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

	err := Cleanup(context.Background(), "/tmp/fake-repo-root", "/nonexistent/workspace/path")
	require.NoError(t, err, "Cleanup must return nil for a non-existent workspace path")
}

func TestCleanup_RemovesDirectory_WhenRepairFails(t *testing.T) {
	t.Parallel()

	// Create a temp directory simulating a workspace that the host user owns.
	// Docker repair will fail (no Docker in unit tests), but os.RemoveAll
	// must still run and clean up the directory.
	wsPath := filepath.Join(t.TempDir(), "workspace")
	require.NoError(t, os.MkdirAll(filepath.Join(wsPath, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wsPath, "subdir", "file.txt"), []byte("data"), 0o644))

	err := Cleanup(context.Background(), "/tmp/fake-repo-root", wsPath)
	require.NoError(t, err)

	_, statErr := os.Stat(wsPath)
	require.True(t, os.IsNotExist(statErr), "workspace directory must be removed even when repair fails")
}

func TestCleanup_Idempotent_AfterRepairFailure(t *testing.T) {
	t.Parallel()

	wsPath := filepath.Join(t.TempDir(), "workspace")
	require.NoError(t, os.MkdirAll(wsPath, 0o755))

	// First call: repair fails, but directory is removed.
	require.NoError(t, Cleanup(context.Background(), "/tmp/fake-repo-root", wsPath))
	// Second call: path is already gone — must return nil.
	require.NoError(t, Cleanup(context.Background(), "/tmp/fake-repo-root", wsPath))
}
