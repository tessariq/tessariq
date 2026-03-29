package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenLogs_CreatesBothFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lf, err := OpenLogs(dir)
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
	lf, err := OpenLogs(dir)
	require.NoError(t, err)
	defer lf.Close()

	_, err = lf.RunLog.WriteString("agent output\n")
	require.NoError(t, err)
	_, err = lf.RunnerLog.WriteString("lifecycle event\n")
	require.NoError(t, err)
}

func TestOpenLogs_CloseIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lf, err := OpenLogs(dir)
	require.NoError(t, err)

	require.NoError(t, lf.Close())
	// second close should not panic or error fatally
	_ = lf.Close()
}

func TestOpenLogs_NonexistentDir(t *testing.T) {
	t.Parallel()

	_, err := OpenLogs(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}

func TestOpenLogs_ContentPersistsAfterClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lf, err := OpenLogs(dir)
	require.NoError(t, err)

	_, err = lf.RunLog.WriteString("hello run\n")
	require.NoError(t, err)
	_, err = lf.RunnerLog.WriteString("hello runner\n")
	require.NoError(t, err)
	require.NoError(t, lf.Close())

	runData, err := os.ReadFile(filepath.Join(dir, "run.log"))
	require.NoError(t, err)
	require.Equal(t, "hello run\n", string(runData))

	runnerData, err := os.ReadFile(filepath.Join(dir, "runner.log"))
	require.NoError(t, err)
	require.Equal(t, "hello runner\n", string(runnerData))
}
