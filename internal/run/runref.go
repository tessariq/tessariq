package run

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	// ErrRunNotFound is returned when a run ref cannot be resolved to an index entry.
	ErrRunNotFound = errors.New("run not found")
	// ErrEmptyIndex is returned when the index has no entries.
	ErrEmptyIndex = errors.New("run index is empty")
	// ErrInvalidRunRef is returned when the ref format is not recognized.
	ErrInvalidRunRef = errors.New("invalid run ref")
)

// ResolveRunRef resolves a run reference against the repository's index.
// Supported formats: explicit run_id, "last", "last-N" (0-indexed from end).
func ResolveRunRef(runsDir, ref string) (IndexEntry, error) {
	if ref == "last" {
		return resolveLastN(runsDir, 0)
	}

	if strings.HasPrefix(ref, "last-") {
		nStr := strings.TrimPrefix(ref, "last-")
		n, err := strconv.Atoi(nStr)
		if err != nil || n < 0 {
			return IndexEntry{}, fmt.Errorf("%w: %s", ErrInvalidRunRef, ref)
		}
		return resolveLastN(runsDir, n)
	}

	if IsValidRunID(ref) {
		return resolveByID(runsDir, ref)
	}

	return IndexEntry{}, fmt.Errorf("%w: %s", ErrInvalidRunRef, ref)
}

func resolveLastN(runsDir string, n int) (IndexEntry, error) {
	entries, err := ReadIndex(runsDir)
	if err != nil {
		return IndexEntry{}, err
	}
	if len(entries) == 0 {
		return IndexEntry{}, ErrEmptyIndex
	}

	idx := len(entries) - 1 - n
	if idx < 0 {
		return IndexEntry{}, fmt.Errorf("%w: last-%d exceeds index size %d", ErrRunNotFound, n, len(entries))
	}

	return entries[idx], nil
}

func resolveByID(runsDir, runID string) (IndexEntry, error) {
	entries, err := ReadIndex(runsDir)
	if err != nil {
		return IndexEntry{}, err
	}

	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.RunID == runID {
			return e, nil
		}
	}

	return IndexEntry{}, fmt.Errorf("%w: %s", ErrRunNotFound, runID)
}
