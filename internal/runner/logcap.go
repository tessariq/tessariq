package runner

import (
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	// DefaultLogCapBytes is the maximum size for capped log files (50 MiB).
	DefaultLogCapBytes int64 = 50 * 1024 * 1024

	// TruncationMarker is appended to log files that exceed the cap.
	TruncationMarker = "\n[truncated]\n"
)

// CappedWriter is a thread-safe io.Writer that enforces a byte cap.
// Once the cap is reached it appends a truncation marker and silently
// discards all subsequent writes.
type CappedWriter struct {
	mu       sync.Mutex
	inner    io.Writer
	maxBytes int64
	written  int64
	capped   bool
}

// NewCappedWriter creates a writer that caps output at maxBytes.
func NewCappedWriter(inner io.Writer, maxBytes int64) *CappedWriter {
	return &CappedWriter{inner: inner, maxBytes: maxBytes}
}

// Write implements io.Writer. After the cap is reached, writes are
// silently discarded (returning len(p), nil) to avoid breaking
// upstream io.Copy or exec.Cmd pipelines.
func (cw *CappedWriter) Write(p []byte) (int, error) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.capped {
		return len(p), nil
	}

	remaining := cw.maxBytes - cw.written
	if int64(len(p)) <= remaining {
		n, err := cw.inner.Write(p)
		cw.written += int64(n)
		return n, err
	}

	// Partial write up to the cap.
	if remaining > 0 {
		n, err := cw.inner.Write(p[:remaining])
		cw.written += int64(n)
		if err != nil {
			return n, err
		}
	}

	// Append the truncation marker and cap further writes.
	_, _ = io.WriteString(cw.inner, TruncationMarker)
	cw.capped = true

	return len(p), nil
}

// Capped reports whether the writer has been truncated.
func (cw *CappedWriter) Capped() bool {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	return cw.capped
}

// CapLogFile truncates a log file in-place if it exceeds maxBytes,
// appending a truncation marker. Returns whether the file was truncated.
//
// Note: a concurrent writer holding an independent fd keeps its own file
// offset, which is NOT updated by this truncate. Its next write will land
// at its stale offset and may create a sparse hole past maxBytes,
// re-growing the file. Callers MUST stop concurrent writers promptly
// after CapLogFile returns truncated == true (see
// startDetachedLogCapMonitor, which calls StopLogStream for this reason).
func CapLogFile(path string, maxBytes int64) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat log file: %w", err)
	}

	if info.Size() <= maxBytes {
		return false, nil
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return false, fmt.Errorf("open log file for truncation: %w", err)
	}
	defer f.Close()

	if err := f.Truncate(maxBytes); err != nil {
		return false, fmt.Errorf("truncate log file: %w", err)
	}

	if _, err := f.Seek(0, 2); err != nil {
		return false, fmt.Errorf("seek to end: %w", err)
	}

	if _, err := f.WriteString(TruncationMarker); err != nil {
		return false, fmt.Errorf("write truncation marker: %w", err)
	}

	return true, nil
}
