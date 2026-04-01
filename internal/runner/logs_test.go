package runner

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenLogs_FilePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lf, err := OpenLogs(dir, 0)
	require.NoError(t, err)
	defer lf.Close()

	runInfo, err := os.Stat(filepath.Join(dir, "run.log"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), runInfo.Mode().Perm(),
		"run.log must be owner-only read/write")

	runnerInfo, err := os.Stat(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), runnerInfo.Mode().Perm(),
		"runner.log must be owner-only read/write")
}

func TestOpenLogs_CreatesBothFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lf, err := OpenLogs(dir, 0)
	require.NoError(t, err)
	defer lf.Close()

	_, err = os.Stat(filepath.Join(dir, "run.log"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
}

func TestOpenLogs_FilesAreWritable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lf, err := OpenLogs(dir, 0)
	require.NoError(t, err)
	defer lf.Close()

	_, err = io.WriteString(lf.RunLog, "agent output\n")
	require.NoError(t, err)
	_, err = io.WriteString(lf.RunnerLog, "lifecycle event\n")
	require.NoError(t, err)
}

func TestOpenLogs_CloseIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lf, err := OpenLogs(dir, 0)
	require.NoError(t, err)

	require.NoError(t, lf.Close())
	// second close should not panic or error fatally
	_ = lf.Close()
}

func TestOpenLogs_NonexistentDir(t *testing.T) {
	t.Parallel()

	_, err := OpenLogs(filepath.Join(t.TempDir(), "missing"), 0)
	require.Error(t, err)
}

func TestOpenLogs_ContentPersistsAfterClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lf, err := OpenLogs(dir, 0)
	require.NoError(t, err)

	_, err = io.WriteString(lf.RunLog, "hello run\n")
	require.NoError(t, err)
	_, err = io.WriteString(lf.RunnerLog, "hello runner\n")
	require.NoError(t, err)
	require.NoError(t, lf.Close())

	runData, err := os.ReadFile(filepath.Join(dir, "run.log"))
	require.NoError(t, err)
	require.Equal(t, "hello run\n", string(runData))

	runnerData, err := os.ReadFile(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
	require.Equal(t, "hello runner\n", string(runnerData))
}
