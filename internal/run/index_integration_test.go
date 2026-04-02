//go:build integration

package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendIndex_ConcurrentAppends(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	const workers = 2
	const entriesPerWorker = 50

	errs := make(chan error, workers*entriesPerWorker)
	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < entriesPerWorker; i++ {
				entry := IndexEntry{
					RunID:         fmt.Sprintf("WORKER%d_ENTRY%03d", workerID, i),
					CreatedAt:     "2026-01-01T00:00:00Z",
					TaskPath:      "t.md",
					TaskTitle:     "t",
					Agent:         "claude-code",
					WorkspaceMode: "worktree",
					State:         "success",
					EvidencePath:  fmt.Sprintf(".tessariq/runs/W%dE%03d", workerID, i),
				}
				if err := AppendIndex(dir, entry); err != nil {
					errs <- err
				}
			}
		}(w)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("append failed: %v", err)
	}

	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	require.Len(t, entries, workers*entriesPerWorker,
		"expected %d entries, got %d", workers*entriesPerWorker, len(entries))
}

func TestResolveRunRef_LastNDeduplicatesWithFileBackedIndex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write entries via AppendIndex (real file I/O with locking).
	// RUN_A running → RUN_B running → RUN_B success
	a := sampleEntry(testRunID1)
	a.State = "running"
	bRunning := sampleEntry(testRunID2)
	bRunning.State = "running"
	bSuccess := sampleEntry(testRunID2)
	bSuccess.State = "success"

	require.NoError(t, AppendIndex(dir, a))
	require.NoError(t, AppendIndex(dir, bRunning))
	require.NoError(t, AppendIndex(dir, bSuccess))

	// Verify raw line count = 3 but unique runs = 2.
	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	require.Len(t, entries, 3, "raw index should have 3 lines")

	// last → B success (newest unique run, latest entry)
	got, err := ResolveRunRef(dir, "last")
	require.NoError(t, err)
	require.Equal(t, bSuccess, got)

	// last-1 → A (previous unique run)
	got, err = ResolveRunRef(dir, "last-1")
	require.NoError(t, err)
	require.Equal(t, a, got)

	// last-2 → out of range (only 2 unique runs)
	_, err = ResolveRunRef(dir, "last-2")
	require.ErrorIs(t, err, ErrRunNotFound)
}

func TestReadIndex_CorruptedRecovery(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write valid entries and corrupt lines.
	valid1 := sampleEntry(testRunID1)
	valid2 := sampleEntry(testRunID2)

	v1JSON, err := json.Marshal(valid1)
	require.NoError(t, err)
	v2JSON, err := json.Marshal(valid2)
	require.NoError(t, err)

	content := string(v1JSON) + "\n" +
		"CORRUPT LINE\n" +
		"\x00\x01\x02\n" +
		string(v2JSON) + "\n" +
		"another bad line\n"

	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(content), 0o600))

	// ReadIndex should recover valid entries.
	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, valid1, entries[0])
	require.Equal(t, valid2, entries[1])

	// Resolution should work despite corruption.
	got, err := ResolveRunRef(dir, "last")
	require.NoError(t, err)
	require.Equal(t, valid2, got)

	got, err = ResolveRunRef(dir, testRunID1)
	require.NoError(t, err)
	require.Equal(t, valid1, got)
}
