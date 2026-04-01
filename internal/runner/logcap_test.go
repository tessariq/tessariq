package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// --- CappedWriter tests ---

func TestCappedWriter_UnderCap(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 100)

	n, err := cw.Write([]byte("short"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "short", buf.String())
	require.False(t, cw.Capped())
}

func TestCappedWriter_ExactlyAtCap(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 10)

	n, err := cw.Write([]byte("0123456789"))
	require.NoError(t, err)
	require.Equal(t, 10, n)
	require.Equal(t, "0123456789", buf.String())
	require.False(t, cw.Capped())
}

func TestCappedWriter_OneByteOverCap(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 10)

	n, err := cw.Write([]byte("01234567890"))
	require.NoError(t, err)
	require.Equal(t, 11, n)
	require.True(t, cw.Capped())

	data := buf.String()
	require.Equal(t, 10+len(TruncationMarker), len(data))
	require.Equal(t, "0123456789", data[:10])
	require.True(t, strings.HasSuffix(data, TruncationMarker))
}

func TestCappedWriter_WellOverCap(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 50)

	payload := strings.Repeat("x", 10000)
	n, err := cw.Write([]byte(payload))
	require.NoError(t, err)
	require.Equal(t, 10000, n)
	require.True(t, cw.Capped())

	data := buf.String()
	require.Equal(t, 50+len(TruncationMarker), len(data))
	require.True(t, strings.HasSuffix(data, TruncationMarker))
}

func TestCappedWriter_MultipleWritesAccumulateOverCap(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 20)

	// First write: 15 bytes (under cap)
	n, err := cw.Write([]byte(strings.Repeat("a", 15)))
	require.NoError(t, err)
	require.Equal(t, 15, n)
	require.False(t, cw.Capped())

	// Second write: 10 bytes (would exceed cap at 25)
	n, err = cw.Write([]byte(strings.Repeat("b", 10)))
	require.NoError(t, err)
	require.Equal(t, 10, n)
	require.True(t, cw.Capped())

	data := buf.String()
	require.Equal(t, 20+len(TruncationMarker), len(data))
	require.Equal(t, strings.Repeat("a", 15)+strings.Repeat("b", 5), data[:20])
	require.True(t, strings.HasSuffix(data, TruncationMarker))
}

func TestCappedWriter_WritesAfterCapDiscarded(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 10)

	_, _ = cw.Write([]byte(strings.Repeat("x", 20)))
	require.True(t, cw.Capped())
	sizeAfterCap := buf.Len()

	// Further writes must not grow the buffer.
	n, err := cw.Write([]byte("more data"))
	require.NoError(t, err)
	require.Equal(t, 9, n)
	require.Equal(t, sizeAfterCap, buf.Len())
}

func TestCappedWriter_MarkerWrittenOnce(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 10)

	// Trigger cap.
	_, _ = cw.Write([]byte(strings.Repeat("x", 20)))
	// Write more to ensure marker doesn't duplicate.
	_, _ = cw.Write([]byte("extra"))
	_, _ = cw.Write([]byte("more"))

	data := buf.String()
	count := strings.Count(data, TruncationMarker)
	require.Equal(t, 1, count, "truncation marker must appear exactly once")
}

func TestCappedWriter_ZeroLengthWrite(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 100)

	n, err := cw.Write([]byte{})
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.Equal(t, 0, buf.Len())
	require.False(t, cw.Capped())
}

func TestCappedWriter_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 500)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cw.Write([]byte(strings.Repeat("g", 20)))
		}()
	}
	wg.Wait()

	// Total attempted: 50 * 20 = 1000 bytes > 500 cap.
	require.True(t, cw.Capped())
	data := buf.String()
	require.LessOrEqual(t, len(data), 500+len(TruncationMarker))
	require.True(t, strings.HasSuffix(data, TruncationMarker))
}
