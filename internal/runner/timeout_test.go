package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteTimeoutFlag_CreatesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, WriteTimeoutFlag(dir))

	_, err := os.Stat(filepath.Join(dir, "timeout.flag"))
	require.NoError(t, err)
}

func TestWriteTimeoutFlag_Idempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, WriteTimeoutFlag(dir))
	require.NoError(t, WriteTimeoutFlag(dir))

	_, err := os.Stat(filepath.Join(dir, "timeout.flag"))
	require.NoError(t, err)
}

func TestWriteTimeoutFlag_NonexistentDir(t *testing.T) {
	t.Parallel()

	err := WriteTimeoutFlag(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}
