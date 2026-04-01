package runner

import (
	"fmt"
	"os"
)

const (
	// DefaultLogCapBytes is the maximum size for capped log files (50 MiB).
	DefaultLogCapBytes int64 = 50 * 1024 * 1024

	// TruncationMarker is appended to log files that exceed the cap.
	TruncationMarker = "\n[truncated]\n"
)

// CapLogFile truncates a log file in-place if it exceeds maxBytes,
// appending a truncation marker. Returns whether the file was truncated.
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
