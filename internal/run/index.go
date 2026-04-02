package run

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// IndexEntry represents one line in index.jsonl.
type IndexEntry struct {
	RunID         string `json:"run_id"`
	CreatedAt     string `json:"created_at"`
	TaskPath      string `json:"task_path"`
	TaskTitle     string `json:"task_title"`
	Agent         string `json:"agent"`
	WorkspaceMode string `json:"workspace_mode"`
	State         string `json:"state"`
	EvidencePath  string `json:"evidence_path"`
}

// isComplete returns true when all required index fields are non-empty.
func (e IndexEntry) isComplete() bool {
	return e.RunID != "" &&
		e.CreatedAt != "" &&
		e.TaskPath != "" &&
		e.TaskTitle != "" &&
		e.Agent != "" &&
		e.WorkspaceMode != "" &&
		e.State != "" &&
		e.EvidencePath != ""
}

// IndexEntryFromManifest builds an IndexEntry from a Manifest and terminal state.
func IndexEntryFromManifest(m Manifest, state string) IndexEntry {
	return IndexEntry{
		RunID:         m.RunID,
		CreatedAt:     m.CreatedAt,
		TaskPath:      m.TaskPath,
		TaskTitle:     m.TaskTitle,
		Agent:         m.Agent,
		WorkspaceMode: m.WorkspaceMode,
		State:         state,
		EvidencePath:  filepath.Join(".tessariq", "runs", m.RunID),
	}
}

// IndexPath returns the path to index.jsonl in the runs directory.
func IndexPath(runsDir string) string {
	return filepath.Join(runsDir, "index.jsonl")
}

// AppendIndex appends a single index entry as a JSON line to index.jsonl.
// Uses file locking and explicit seek for concurrent append safety.
func AppendIndex(runsDir string, entry IndexEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal index entry: %w", err)
	}
	data = append(data, '\n')

	path := IndexPath(runsDir)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open index file: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock index file: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek index file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write index entry: %w", err)
	}

	return nil
}

// ReadIndex reads all valid entries from index.jsonl.
// Malformed JSON lines are silently skipped.
// Returns an empty slice (not an error) when the file does not exist.
func ReadIndex(runsDir string) ([]IndexEntry, error) {
	path := IndexPath(runsDir)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open index file: %w", err)
	}
	defer f.Close()

	var entries []IndexEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry IndexEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // skip malformed lines
		}
		if !entry.isComplete() {
			continue // skip entries missing required fields
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return entries, fmt.Errorf("read index file: %w", err)
	}

	return entries, nil
}
