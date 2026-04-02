package promote

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
)

func TestDefaultBranchName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "tessariq/01ARZ3NDEKTSV4RRFFQ69G5FAV", defaultBranchName("01ARZ3NDEKTSV4RRFFQ69G5FAV"))
}

func TestDefaultCommitMessage_UsesTaskTitle(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Implement promote", defaultCommitMessage("Implement promote", "RUN123"))
}

func TestDefaultCommitMessage_FallsBackToRunID(t *testing.T) {
	t.Parallel()

	require.Equal(t, "tessariq: apply run RUN123", defaultCommitMessage("", "RUN123"))
}

func TestBuildCommitMessage_WithDefaultTrailers(t *testing.T) {
	t.Parallel()

	manifest := run.Manifest{
		RunID:    "RUN123",
		BaseSHA:  "abc123",
		TaskPath: "tasks/sample.md",
	}

	require.Equal(t, "Implement promote\n\nTessariq-Run: RUN123\nTessariq-Base: abc123\nTessariq-Task: tasks/sample.md\n", buildCommitMessage("Implement promote", manifest, true))
}

func TestBuildCommitMessage_WithoutTrailers(t *testing.T) {
	t.Parallel()

	manifest := run.Manifest{
		RunID:    "RUN123",
		BaseSHA:  "abc123",
		TaskPath: "tasks/sample.md",
	}

	require.Equal(t, "Implement promote\n", buildCommitMessage("Implement promote", manifest, false))
}

func TestResolveBranchName_UsesOverride(t *testing.T) {
	t.Parallel()

	require.Equal(t, "feature/custom", resolveBranchName("RUN123", "feature/custom"))
}

func TestResolveBranchName_UsesDefaultWhenUnset(t *testing.T) {
	t.Parallel()

	require.Equal(t, "tessariq/RUN123", resolveBranchName("RUN123", ""))
}

func TestResolveCommitMessage_UsesOverride(t *testing.T) {
	t.Parallel()

	require.Equal(t, "custom message", resolveCommitMessage(run.Manifest{RunID: "RUN123", TaskTitle: "ignored"}, "custom message"))
}

func TestResolveCommitMessage_UsesManifestDefaults(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Task Title", resolveCommitMessage(run.Manifest{RunID: "RUN123", TaskTitle: "Task Title"}, ""))
	require.Equal(t, "tessariq: apply run RUN123", resolveCommitMessage(run.Manifest{RunID: "RUN123"}, ""))
}

func TestHasNonEmptyFile_Missing(t *testing.T) {
	t.Parallel()

	ok, err := hasNonEmptyFile(filepath.Join(t.TempDir(), "nonexistent.txt"), "nonexistent.txt")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestHasNonEmptyFile_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.txt"), []byte{}, 0o600))

	ok, err := hasNonEmptyFile(filepath.Join(dir, "empty.txt"), "empty.txt")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestHasNonEmptyFile_Present(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.txt"), []byte("content"), 0o600))

	ok, err := hasNonEmptyFile(filepath.Join(dir, "data.txt"), "data.txt")
	require.NoError(t, err)
	require.True(t, ok)
}
