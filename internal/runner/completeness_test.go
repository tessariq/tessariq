package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckEvidenceCompleteness_AllPresent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := []string{
		"manifest.json", "status.json", "agent.json", "runtime.json",
		"task.md", "run.log", "runner.log", "workspace.json",
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("data"), 0o600))
	}

	err := CheckEvidenceCompleteness(dir)
	require.NoError(t, err)
}

func TestCheckEvidenceCompleteness_MissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Write all except status.json.
	files := []string{
		"manifest.json", "agent.json", "runtime.json",
		"task.md", "run.log", "runner.log", "workspace.json",
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("data"), 0o600))
	}

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "status.json")
}

func TestCheckEvidenceCompleteness_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := []string{
		"manifest.json", "status.json", "agent.json", "runtime.json",
		"task.md", "run.log", "runner.log", "workspace.json",
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("data"), 0o600))
	}
	// Make one file empty.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte{}, 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "manifest.json")
}

func TestCheckEvidenceCompleteness_MultipleMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Write only a few files.
	for _, f := range []string{"manifest.json", "run.log"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("data"), 0o600))
	}

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
}
