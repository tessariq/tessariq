package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapLogFile_WellUnderLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	content := "short log content"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	truncated, err := CapLogFile(path, 1024)
	require.NoError(t, err)
	require.False(t, truncated)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, content, string(data))
}

func TestCapLogFile_ExactlyAtLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	content := strings.Repeat("x", 100)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	truncated, err := CapLogFile(path, 100)
	require.NoError(t, err)
	require.False(t, truncated)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, content, string(data))
}

func TestCapLogFile_OneByteOverLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	content := strings.Repeat("x", 101)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	truncated, err := CapLogFile(path, 100)
	require.NoError(t, err)
	require.True(t, truncated)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 100+len(TruncationMarker), len(data))
	require.True(t, strings.HasSuffix(string(data), TruncationMarker))
	require.Equal(t, strings.Repeat("x", 100), string(data[:100]))
}

func TestCapLogFile_WellOverLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	content := strings.Repeat("a", 10000)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	truncated, err := CapLogFile(path, 500)
	require.NoError(t, err)
	require.True(t, truncated)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 500+len(TruncationMarker), len(data))
	require.True(t, strings.HasSuffix(string(data), TruncationMarker))
}

func TestCapLogFile_NonexistentFile(t *testing.T) {
	t.Parallel()

	_, err := CapLogFile("/nonexistent/path/test.log", 100)
	require.Error(t, err)
}

func TestCapLogFile_PreservesPermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	content := strings.Repeat("x", 200)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := CapLogFile(path, 100)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestCapLogFile_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	require.NoError(t, os.WriteFile(path, []byte{}, 0o600))

	truncated, err := CapLogFile(path, 100)
	require.NoError(t, err)
	require.False(t, truncated)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Empty(t, data)
}
