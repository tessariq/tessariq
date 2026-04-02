package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Valid 26-char Crockford Base32 run IDs for testing.
const (
	testRunID1 = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	testRunID2 = "01BRZ3NDEKTSV4RRFFQ69G5FAV"
	testRunID3 = "01CRZ3NDEKTSV4RRFFQ69G5FAV"
)

func sampleEntry(runID string) IndexEntry {
	return IndexEntry{
		RunID:         runID,
		CreatedAt:     "2026-01-27T12:00:00Z",
		TaskPath:      "specs/example.md",
		TaskTitle:     "Example task",
		Agent:         "claude-code",
		WorkspaceMode: "worktree",
		State:         "success",
		EvidencePath:  ".tessariq/runs/" + runID,
	}
}

func TestIndexEntryFromManifest(t *testing.T) {
	t.Parallel()
	m := Manifest{
		SchemaVersion: 1,
		RunID:         testRunID1,
		TaskPath:      "specs/task.md",
		TaskTitle:     "My task",
		Agent:         "claude-code",
		WorkspaceMode: "worktree",
		CreatedAt:     "2026-03-01T10:00:00Z",
	}
	entry := IndexEntryFromManifest(m, "success")
	require.Equal(t, m.RunID, entry.RunID)
	require.Equal(t, m.CreatedAt, entry.CreatedAt)
	require.Equal(t, m.TaskPath, entry.TaskPath)
	require.Equal(t, m.TaskTitle, entry.TaskTitle)
	require.Equal(t, m.Agent, entry.Agent)
	require.Equal(t, m.WorkspaceMode, entry.WorkspaceMode)
	require.Equal(t, "success", entry.State)
	require.Equal(t, ".tessariq/runs/"+m.RunID, entry.EvidencePath)
}

func TestAppendIndex_CreatesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	entry := sampleEntry(testRunID1)

	err := AppendIndex(dir, entry)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "index.jsonl"))
	require.NoError(t, err)

	var got IndexEntry
	require.NoError(t, json.Unmarshal(data[:len(data)-1], &got)) // strip trailing newline
	require.Equal(t, entry, got)
}

func TestAppendIndex_AppendsLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	entry1 := sampleEntry(testRunID1)
	entry2 := sampleEntry(testRunID2)

	require.NoError(t, AppendIndex(dir, entry1))
	require.NoError(t, AppendIndex(dir, entry2))

	data, err := os.ReadFile(filepath.Join(dir, "index.jsonl"))
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 2)

	var got1, got2 IndexEntry
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &got1))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &got2))
	require.Equal(t, entry1, got1)
	require.Equal(t, entry2, got2)
}

func TestAppendIndex_FilePermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	entry := sampleEntry(testRunID1)

	require.NoError(t, AppendIndex(dir, entry))

	info, err := os.Stat(filepath.Join(dir, "index.jsonl"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestReadIndex_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create empty file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(""), 0o600))

	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestReadIndex_FileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestReadIndex_ValidEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	entry1 := sampleEntry(testRunID1)
	entry2 := sampleEntry(testRunID2)

	require.NoError(t, AppendIndex(dir, entry1))
	require.NoError(t, AppendIndex(dir, entry2))

	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, entry1, entries[0])
	require.Equal(t, entry2, entries[1])
}

func TestReadIndex_SkipsMalformedAndIncompleteLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	valid := sampleEntry(testRunID1)
	validJSON, err := json.Marshal(valid)
	require.NoError(t, err)

	content := string(validJSON) + "\n" +
		"this is not json\n" +
		"{\"run_id\":\"MISSING_FIELDS\"}\n" // valid JSON but missing required fields — filtered
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(content), 0o600))

	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1) // only the fully complete entry
	require.Equal(t, valid, entries[0])
}

func TestIndexEntry_IsComplete(t *testing.T) {
	t.Parallel()

	full := sampleEntry(testRunID1)
	require.True(t, full.isComplete(), "all fields present must be complete")

	tests := []struct {
		name   string
		mutate func(IndexEntry) IndexEntry
	}{
		{"missing_run_id", func(e IndexEntry) IndexEntry { e.RunID = ""; return e }},
		{"missing_created_at", func(e IndexEntry) IndexEntry { e.CreatedAt = ""; return e }},
		{"missing_task_path", func(e IndexEntry) IndexEntry { e.TaskPath = ""; return e }},
		{"missing_task_title", func(e IndexEntry) IndexEntry { e.TaskTitle = ""; return e }},
		{"missing_agent", func(e IndexEntry) IndexEntry { e.Agent = ""; return e }},
		{"missing_workspace_mode", func(e IndexEntry) IndexEntry { e.WorkspaceMode = ""; return e }},
		{"missing_state", func(e IndexEntry) IndexEntry { e.State = ""; return e }},
		{"missing_evidence_path", func(e IndexEntry) IndexEntry { e.EvidencePath = ""; return e }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entry := tc.mutate(sampleEntry(testRunID1))
			require.False(t, entry.isComplete(), "entry with %s must be incomplete", tc.name)
		})
	}

	t.Run("all_empty", func(t *testing.T) {
		t.Parallel()
		require.False(t, IndexEntry{}.isComplete(), "zero-value entry must be incomplete")
	})
}

func TestReadIndex_SkipsIncompleteEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	valid := sampleEntry(testRunID1)
	validJSON, err := json.Marshal(valid)
	require.NoError(t, err)

	// Partial entries: one with only run_id, one with run_id+created_at.
	partial1 := `{"run_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV"}`
	partial2 := `{"run_id":"01BRZ3NDEKTSV4RRFFQ69G5FAV","created_at":"2026-01-27T12:00:00Z"}`
	content := string(validJSON) + "\n" + partial1 + "\n" + partial2 + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(content), 0o600))

	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, valid, entries[0])
}

func TestIndexPath(t *testing.T) {
	t.Parallel()
	require.Equal(t, "/runs/index.jsonl", IndexPath("/runs"))
}
