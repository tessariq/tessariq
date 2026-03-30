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
	applied := map[string]bool{
		"model":       false,
		"interactive": true,
	}

	info := NewAgentInfo("claude-code", requested, applied)

	require.Equal(t, 1, info.SchemaVersion)
	require.Equal(t, "claude-code", info.Agent)
	require.Equal(t, requested, info.Requested)
	require.Equal(t, applied, info.Applied)
}

func TestNewAgentInfo_RecordsUnsupportedRequested(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "gpt-5.4",
		"interactive": true,
	}
	applied := map[string]bool{
		"model":       false,
		"interactive": true,
	}

	info := NewAgentInfo("claude-code", requested, applied)

	require.Equal(t, "gpt-5.4", info.Requested["model"])
	require.False(t, info.Applied["model"])
}

func TestNewAgentInfo_AppliedDiffersFromRequested(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "o3-pro",
		"interactive": false,
	}
	applied := map[string]bool{
		"model":       false,
		"interactive": false,
	}

	info := NewAgentInfo("opencode", requested, applied)

	require.Equal(t, "o3-pro", info.Requested["model"],
		"requested value must be preserved even when not applied")
	require.False(t, info.Applied["model"],
		"applied must reflect actual agent capability")
	require.False(t, info.Applied["interactive"])
}

func TestNewAgentInfo_Extensibility(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "gpt-5.4",
		"interactive": true,
		"custom_flag": "extra-value",
	}
	applied := map[string]bool{
		"model":       true,
		"interactive": true,
		"custom_flag": false,
	}

	info := NewAgentInfo("claude-code", requested, applied)

	require.Equal(t, 1, info.SchemaVersion,
		"extra options must not change schema_version")
	require.Equal(t, "extra-value", info.Requested["custom_flag"])
	require.False(t, info.Applied["custom_flag"])
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
		"applied":        true,
	}

	for k := range raw {
		require.True(t, expectedKeys[k], "unexpected key: %s", k)
	}
	require.Len(t, raw, len(expectedKeys))
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

	var applied map[string]bool
	require.NoError(t, json.Unmarshal(raw["applied"], &applied))
	require.False(t, applied["model"])
	require.True(t, applied["interactive"])
}
