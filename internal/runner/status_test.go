package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestState_IsTerminal(t *testing.T) {
	t.Parallel()

	terminal := []State{StateSuccess, StateFailed, StateTimeout, StateKilled, StateInterrupted}
	for _, s := range terminal {
		require.True(t, s.IsTerminal(), "expected %q to be terminal", s)
	}

	require.False(t, StateRunning.IsTerminal(), "running should not be terminal")
}

func TestNewTerminalStatus_RequiredFields(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	finished := time.Date(2026, 3, 29, 12, 10, 0, 0, time.UTC)

	s := NewTerminalStatus(StateSuccess, started, finished, 0, false)

	require.Equal(t, 1, s.SchemaVersion)
	require.Equal(t, StateSuccess, s.State)
	require.Equal(t, "2026-03-29T12:00:00Z", s.StartedAt)
	require.Equal(t, "2026-03-29T12:10:00Z", s.FinishedAt)
	require.Equal(t, 0, s.ExitCode)
	require.False(t, s.TimedOut)
}

func TestNewTerminalStatus_AllStates(t *testing.T) {
	t.Parallel()

	now := time.Now()
	states := []struct {
		state    State
		exitCode int
		timedOut bool
	}{
		{StateSuccess, 0, false},
		{StateFailed, 1, false},
		{StateTimeout, -1, true},
		{StateKilled, -1, false},
		{StateInterrupted, -1, false},
	}

	for _, tc := range states {
		s := NewTerminalStatus(tc.state, now, now, tc.exitCode, tc.timedOut)
		require.Equal(t, tc.state, s.State)
		require.Equal(t, tc.exitCode, s.ExitCode)
		require.Equal(t, tc.timedOut, s.TimedOut)
	}
}

func TestNewInitialStatus_NonTerminal(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	s := NewInitialStatus(started)

	require.Equal(t, 1, s.SchemaVersion)
	require.Equal(t, StateRunning, s.State)
	require.Equal(t, "2026-03-29T12:00:00Z", s.StartedAt)
	require.Empty(t, s.FinishedAt)
	require.Equal(t, 0, s.ExitCode)
	require.False(t, s.TimedOut)
}

func TestStatus_JSONSerialization(t *testing.T) {
	t.Parallel()

	s := Status{
		SchemaVersion: 1,
		State:         StateSuccess,
		StartedAt:     "2026-03-29T12:00:00Z",
		FinishedAt:    "2026-03-29T12:10:00Z",
		ExitCode:      0,
		TimedOut:      false,
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	expectedKeys := map[string]bool{
		"schema_version": true,
		"state":          true,
		"started_at":     true,
		"finished_at":    true,
		"exit_code":      true,
		"timed_out":      true,
	}

	for k := range raw {
		require.True(t, expectedKeys[k], "unexpected key: %s", k)
	}
	require.Len(t, raw, len(expectedKeys))
}

func TestStatus_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := Status{
		SchemaVersion: 1,
		State:         StateTimeout,
		StartedAt:     "2026-03-29T12:00:00Z",
		FinishedAt:    "2026-03-29T12:30:00Z",
		ExitCode:      -1,
		TimedOut:      true,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var parsed Status
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, original, parsed)
}

func TestWriteStatus_FilePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s := NewTerminalStatus(StateSuccess, time.Now(), time.Now(), 0, false)

	require.NoError(t, WriteStatus(dir, s))

	info, err := os.Stat(filepath.Join(dir, "status.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"evidence file must be owner-only read/write")
}

func TestWriteStatus_CreatesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s := NewTerminalStatus(StateSuccess, time.Now(), time.Now(), 0, false)

	require.NoError(t, WriteStatus(dir, s))

	data, err := os.ReadFile(filepath.Join(dir, "status.json"))
	require.NoError(t, err)

	var parsed Status
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, s, parsed)
}

func TestWriteStatus_OverwritesExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initial := NewInitialStatus(time.Now())
	require.NoError(t, WriteStatus(dir, initial))

	final := NewTerminalStatus(StateFailed, time.Now(), time.Now(), 1, false)
	require.NoError(t, WriteStatus(dir, final))

	data, err := os.ReadFile(filepath.Join(dir, "status.json"))
	require.NoError(t, err)

	var parsed Status
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, StateFailed, parsed.State)
}

func TestReadStatus_ReadsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	expected := NewTerminalStatus(StateKilled, time.Now(), time.Now(), -1, false)
	require.NoError(t, WriteStatus(dir, expected))

	got, err := ReadStatus(dir)
	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestReadStatus_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := ReadStatus(t.TempDir())
	require.Error(t, err)
}

func TestStatus_Validate(t *testing.T) {
	t.Parallel()

	valid := Status{
		SchemaVersion: 1,
		State:         StateSuccess,
		StartedAt:     "2026-01-01T00:00:00Z",
	}
	require.NoError(t, valid.Validate())

	cases := []struct {
		name    string
		mutate  func(*Status)
		wantErr string
	}{
		{"bad schema_version", func(s *Status) { s.SchemaVersion = 0 }, "schema_version"},
		{"missing state", func(s *Status) { s.State = "" }, "state"},
		{"missing started_at", func(s *Status) { s.StartedAt = "" }, "started_at"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := valid
			tc.mutate(&s)
			err := s.Validate()
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
