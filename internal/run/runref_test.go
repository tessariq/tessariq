package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeIndexEntries(t *testing.T, dir string, entries ...IndexEntry) {
	t.Helper()
	for _, e := range entries {
		require.NoError(t, AppendIndex(dir, e))
	}
}

func TestResolveRunRef_ExplicitRunID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	e1 := sampleEntry(testRunID1)
	e2 := sampleEntry(testRunID2)
	writeIndexEntries(t, dir, e1, e2)

	got, err := ResolveRunRef(dir, testRunID1)
	require.NoError(t, err)
	require.Equal(t, e1, got)
}

func TestResolveRunRef_Last(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	e1 := sampleEntry(testRunID1)
	e2 := sampleEntry(testRunID2)
	writeIndexEntries(t, dir, e1, e2)

	got, err := ResolveRunRef(dir, "last")
	require.NoError(t, err)
	require.Equal(t, e2, got)
}

func TestResolveRunRef_LastN(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	e1 := sampleEntry(testRunID1)
	e2 := sampleEntry(testRunID2)
	e3 := sampleEntry(testRunID3)
	writeIndexEntries(t, dir, e1, e2, e3)

	// last-0 = last entry
	got, err := ResolveRunRef(dir, "last-0")
	require.NoError(t, err)
	require.Equal(t, e3, got)

	// last-1 = second-to-last
	got, err = ResolveRunRef(dir, "last-1")
	require.NoError(t, err)
	require.Equal(t, e2, got)

	// last-2 = third-to-last
	got, err = ResolveRunRef(dir, "last-2")
	require.NoError(t, err)
	require.Equal(t, e1, got)
}

func TestResolveRunRef_LastOnEmptyIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_, err := ResolveRunRef(dir, "last")
	require.ErrorIs(t, err, ErrEmptyIndex)
}

func TestResolveRunRef_LastN_OutOfRange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeIndexEntries(t, dir, sampleEntry(testRunID1))

	_, err := ResolveRunRef(dir, "last-1")
	require.ErrorIs(t, err, ErrRunNotFound)
}

func TestResolveRunRef_InvalidRef(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_, err := ResolveRunRef(dir, "not-a-valid-ref")
	require.ErrorIs(t, err, ErrInvalidRunRef)
}

func TestResolveRunRef_UnknownRunID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeIndexEntries(t, dir, sampleEntry(testRunID1))

	_, err := ResolveRunRef(dir, testRunID2)
	require.ErrorIs(t, err, ErrRunNotFound)
}

func TestResolveRunRef_ExplicitRunIDReturnsLatestEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	earlier := sampleEntry(testRunID1)
	earlier.State = "running"
	later := earlier
	later.State = "success"
	writeIndexEntries(t, dir, earlier, sampleEntry(testRunID2), later)

	got, err := ResolveRunRef(dir, testRunID1)
	require.NoError(t, err)
	require.Equal(t, later, got)
}

func TestResolveRunRef_MalformedLinesSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	valid := sampleEntry(testRunID1)
	validJSON, err := json.Marshal(valid)
	require.NoError(t, err)

	content := "garbage line\n" + string(validJSON) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(content), 0o600))

	got, err := ResolveRunRef(dir, "last")
	require.NoError(t, err)
	require.Equal(t, valid, got)
}

func TestResolveRunRef_LastNegativeN(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_, err := ResolveRunRef(dir, "last--1")
	require.ErrorIs(t, err, ErrInvalidRunRef)
}

func TestResolveRunRef_LastSkipsIncompleteEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	valid := sampleEntry(testRunID1)
	validJSON, err := json.Marshal(valid)
	require.NoError(t, err)

	// Write valid entry first, then two incomplete entries after it.
	incomplete1 := `{"run_id":"` + testRunID2 + `"}`
	incomplete2 := `{"run_id":"` + testRunID3 + `","state":"success"}`
	content := string(validJSON) + "\n" + incomplete1 + "\n" + incomplete2 + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(content), 0o600))

	got, err := ResolveRunRef(dir, "last")
	require.NoError(t, err)
	require.Equal(t, valid, got, "last must resolve to the last complete entry, not the last line")
}

func TestResolveRunRef_OnlyIncompleteEntriesYieldsEmptyIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := `{"run_id":"` + testRunID1 + `"}` + "\n" +
		`{"run_id":"` + testRunID2 + `","state":"success"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(content), 0o600))

	_, err := ResolveRunRef(dir, "last")
	require.ErrorIs(t, err, ErrEmptyIndex)
}

func TestResolveRunRef_ExplicitIDSkipsIncompleteEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write an incomplete entry with a valid run ID.
	content := `{"run_id":"` + testRunID1 + `","state":"success"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(content), 0o600))

	_, err := ResolveRunRef(dir, testRunID1)
	require.ErrorIs(t, err, ErrRunNotFound)
}
