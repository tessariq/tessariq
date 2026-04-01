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
