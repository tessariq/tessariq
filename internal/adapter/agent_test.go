package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAgentInfo_RequiredFields(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "gpt-5.4",
		"interactive": true,
	}
	supported := map[string]bool{
		"model":       false,
		"interactive": true,
	}

	info := NewAgentInfo("claude-code", requested, supported)

	require.Equal(t, 1, info.SchemaVersion)
	require.Equal(t, "claude-code", info.Agent)
	require.Equal(t, requested, info.Requested)
	require.Equal(t, supported, info.Supported)
}

func TestNewAgentInfo_RecordsUnsupportedRequested(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "gpt-5.4",
		"interactive": true,
	}
	supported := map[string]bool{
		"model":       false,
		"interactive": true,
	}

	info := NewAgentInfo("claude-code", requested, supported)

	require.Equal(t, "gpt-5.4", info.Requested["model"])
	require.False(t, info.Supported["model"])
}

func TestNewAgentInfo_SupportedDiffersFromRequested(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "o3-pro",
		"interactive": false,
	}
	supported := map[string]bool{
		"model":       false,
		"interactive": false,
	}

	info := NewAgentInfo("opencode", requested, supported)

	require.Equal(t, "o3-pro", info.Requested["model"],
		"requested value must be preserved even when not supported")
	require.False(t, info.Supported["model"],
		"supported must reflect actual agent capability")
	require.False(t, info.Supported["interactive"])
}

func TestNewAgentInfo_Extensibility(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "gpt-5.4",
		"interactive": true,
		"custom_flag": "extra-value",
	}
	supported := map[string]bool{
		"model":       true,
		"interactive": true,
		"custom_flag": false,
	}

	info := NewAgentInfo("claude-code", requested, supported)

	require.Equal(t, 1, info.SchemaVersion,
		"extra options must not change schema_version")
	require.Equal(t, "extra-value", info.Requested["custom_flag"])
	require.False(t, info.Supported["custom_flag"])
}

func TestNewAgentInfo_ExactlyFourTopLevelKeys(t *testing.T) {
	t.Parallel()

	info := NewAgentInfo("claude-code",
		map[string]any{"model": "x"},
		map[string]bool{"model": true},
	)

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	expectedKeys := map[string]bool{
		"schema_version": true,
		"agent":          true,
		"requested":      true,
		"supported":      true,
	}

	for k := range raw {
		require.True(t, expectedKeys[k], "unexpected key: %s", k)
	}
	require.Len(t, raw, len(expectedKeys))
}

func TestWriteAgentInfo_DirectoryPermissions(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewAgentInfo("claude-code",
		map[string]any{"model": "x"},
		map[string]bool{"model": true},
	)

	require.NoError(t, WriteAgentInfo(dir, info))

	stat, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), stat.Mode().Perm(),
		"evidence directory must be owner-only")
}

func TestWriteAgentInfo_FilePermissions(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewAgentInfo("claude-code",
		map[string]any{"model": "x"},
		map[string]bool{"model": true},
	)

	require.NoError(t, WriteAgentInfo(dir, info))

	stat, err := os.Stat(filepath.Join(dir, "agent.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), stat.Mode().Perm(),
		"evidence file must be owner-only read/write")
}

func TestWriteAgentInfo_CreatesDirectoryAndFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewAgentInfo("claude-code",
		map[string]any{"model": "gpt-5.4"},
		map[string]bool{"model": true},
	)

	require.NoError(t, WriteAgentInfo(dir, info))

	_, err := os.Stat(dir)
	require.NoError(t, err, "evidence directory must be created")

	_, err = os.Stat(filepath.Join(dir, "agent.json"))
	require.NoError(t, err, "agent.json must be created")
}

func TestWriteAgentInfo_WritesValidJSON(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewAgentInfo("claude-code",
		map[string]any{"model": "gpt-5.4", "interactive": true},
		map[string]bool{"model": false, "interactive": true},
	)

	require.NoError(t, WriteAgentInfo(dir, info))

	data, err := os.ReadFile(filepath.Join(dir, "agent.json"))
	require.NoError(t, err)

	var parsed AgentInfo
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, info.SchemaVersion, parsed.SchemaVersion)
	require.Equal(t, info.Agent, parsed.Agent)
}

func TestAgentInfo_Validate(t *testing.T) {
	t.Parallel()

	valid := NewAgentInfo("claude-code", map[string]any{}, map[string]bool{})
	require.NoError(t, valid.Validate())

	cases := []struct {
		name    string
		info    AgentInfo
		wantErr string
	}{
		{"bad schema_version", AgentInfo{SchemaVersion: 0, Agent: "claude-code"}, "schema_version"},
		{"missing agent", AgentInfo{SchemaVersion: 1, Agent: ""}, "agent"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.info.Validate()
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestReadAgentInfo_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	original := NewAgentInfo("claude-code",
		map[string]any{"model": "gpt-5.4"},
		map[string]bool{"model": false},
	)
	require.NoError(t, WriteAgentInfo(dir, original))

	got, err := ReadAgentInfo(dir)
	require.NoError(t, err)
	require.Equal(t, original.SchemaVersion, got.SchemaVersion)
	require.Equal(t, original.Agent, got.Agent)
}

func TestReadAgentInfo_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := ReadAgentInfo(t.TempDir())
	require.Error(t, err)
}

func TestReadAgentInfo_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.json"), []byte("not-json"), 0o600))

	_, err := ReadAgentInfo(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "parse")
}

func TestWriteAgentInfo_JSONMatchesSpecShape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	info := NewAgentInfo("claude-code",
		map[string]any{"model": "gpt-5.4", "interactive": true},
		map[string]bool{"model": false, "interactive": true},
	)

	require.NoError(t, WriteAgentInfo(dir, info))

	data, err := os.ReadFile(filepath.Join(dir, "agent.json"))
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	var schemaVersion int
	require.NoError(t, json.Unmarshal(raw["schema_version"], &schemaVersion))
	require.Equal(t, 1, schemaVersion)

	var agent string
	require.NoError(t, json.Unmarshal(raw["agent"], &agent))
	require.Equal(t, "claude-code", agent)

	var requested map[string]any
	require.NoError(t, json.Unmarshal(raw["requested"], &requested))
	require.Equal(t, "gpt-5.4", requested["model"])
	require.Equal(t, true, requested["interactive"])

	var supported map[string]bool
	require.NoError(t, json.Unmarshal(raw["supported"], &supported))
	require.False(t, supported["model"])
	require.True(t, supported["interactive"])
}
