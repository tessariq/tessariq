//go:build integration || e2e

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

// knownRunID is a valid ULID written into the index by tests that need run-ref
// resolution to run against a non-empty index; unknownRunID is a valid ULID
// that is never indexed.
const (
	knownRunID   = "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	unknownRunID = "01ARZ3NDEKTSV4RRFFQ69G5FAY"
)

// indexEntryFixture builds a complete index entry for runID in the given state,
// with its evidence path pointing at that run's own evidence directory. Tests
// that forge an entry override the field they are forging.
//
// Building the fixture from run.IndexEntry rather than a hand-written JSON
// literal means a change to the index schema breaks compilation here instead of
// silently producing entries that ReadIndex drops as incomplete.
func indexEntryFixture(runID, state string) run.IndexEntry {
	return run.IndexEntry{
		RunID:         runID,
		CreatedAt:     "2026-01-01T00:00:00Z",
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "Sample Task",
		Agent:         "claude-code",
		WorkspaceMode: "worktree",
		State:         state,
		EvidencePath:  filepath.Join(".tessariq", "runs", runID),
	}
}

// writeIndexEntries replaces the repository's index.jsonl with the given
// entries, one JSON line each.
//
// Entries are marshalled on the host but written inside the container: the
// container runs as root, so `.tessariq/` and everything below it is root-owned
// on the host bind mount and a host-side write would be denied.
func writeIndexEntries(t *testing.T, env *containers.RunEnv, entries ...run.IndexEntry) {
	t.Helper()

	var lines strings.Builder
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		require.NoError(t, err, "marshal index entry")
		lines.Write(data)
		lines.WriteByte('\n')
	}

	runsDir := filepath.Join(env.Dir(), "repo", ".tessariq", "runs")
	indexPath := filepath.Join(runsDir, "index.jsonl")
	// The payload is a printf argument, not its format string, so a literal %
	// in an entry cannot be interpreted as a verb.
	cmd := fmt.Sprintf("mkdir -p %s && printf '%%s' '%s' > %s",
		runsDir, strings.ReplaceAll(lines.String(), "'", `'\''`), indexPath)

	code, output, err := env.Exec(context.Background(), []string{"sh", "-c", cmd})
	require.NoError(t, err, "write index entries: %s", output)
	require.Equal(t, 0, code, "write index entries exited %d: %s", code, output)
}
