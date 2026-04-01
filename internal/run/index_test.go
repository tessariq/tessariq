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

func TestReadIndex_SkipsMalformedLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	valid := sampleEntry(testRunID1)
	validJSON, err := json.Marshal(valid)
	require.NoError(t, err)

	content := string(validJSON) + "\n" +
		"this is not json\n" +
		"{\"run_id\":\"MISSING_FIELDS\"}\n" // missing fields but valid JSON — still parsed
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(content), 0o600))

	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	require.Len(t, entries, 2) // valid entry + partial entry (valid JSON)
	require.Equal(t, valid, entries[0])
}

func TestIndexPath(t *testing.T) {
	t.Parallel()
	require.Equal(t, "/runs/index.jsonl", IndexPath("/runs"))
}
