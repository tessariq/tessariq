package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewInfo_RequiredFields(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "gpt-5.4",
		"interactive": true,
	}
	applied := map[string]bool{
		"model":       false,
		"interactive": true,
	}

	info := NewInfo("claude-code", "example/image:tag", requested, applied)

	require.Equal(t, 1, info.SchemaVersion)
	require.Equal(t, "claude-code", info.Adapter)
	require.Equal(t, "example/image:tag", info.Image)
	require.Equal(t, requested, info.Requested)
	require.Equal(t, applied, info.Applied)
}

func TestNewInfo_RecordsUnsupportedRequested(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "gpt-5.4",
		"interactive": true,
	}
	applied := map[string]bool{
		"model":       false,
		"interactive": true,
	}

	info := NewInfo("claude-code", "img:latest", requested, applied)

	require.Equal(t, "gpt-5.4", info.Requested["model"])
	require.False(t, info.Applied["model"])
}

func TestNewInfo_AppliedDiffersFromRequested(t *testing.T) {
	t.Parallel()

	requested := map[string]any{
		"model":       "o3-pro",
		"interactive": false,
	}
	applied := map[string]bool{
		"model":       false,
		"interactive": false,
	}

	info := NewInfo("opencode", "img:v1", requested, applied)

	require.Equal(t, "o3-pro", info.Requested["model"],
		"requested value must be preserved even when not applied")
	require.False(t, info.Applied["model"],
		"applied must reflect actual adapter capability")
	require.False(t, info.Applied["interactive"])
}

func TestNewInfo_Extensibility(t *testing.T) {
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

	info := NewInfo("claude-code", "img:v2", requested, applied)

	require.Equal(t, 1, info.SchemaVersion,
		"extra options must not change schema_version")
	require.Equal(t, "extra-value", info.Requested["custom_flag"])
	require.False(t, info.Applied["custom_flag"])
}

func TestNewInfo_ExactlyFiveTopLevelKeys(t *testing.T) {
	t.Parallel()

	info := NewInfo("claude-code", "img:tag",
		map[string]any{"model": "x"},
		map[string]bool{"model": true},
	)

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	expectedKeys := map[string]bool{
		"schema_version": true,
		"adapter":        true,
		"image":          true,
		"requested":      true,
		"applied":        true,
	}

	for k := range raw {
		require.True(t, expectedKeys[k], "unexpected key: %s", k)
	}
	require.Len(t, raw, len(expectedKeys))
}

func TestWriteInfo_CreatesDirectoryAndFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewInfo("claude-code", "img:tag",
		map[string]any{"model": "gpt-5.4"},
		map[string]bool{"model": true},
	)

	require.NoError(t, WriteInfo(dir, info))

	_, err := os.Stat(dir)
	require.NoError(t, err, "evidence directory must be created")

	_, err = os.Stat(filepath.Join(dir, "adapter.json"))
	require.NoError(t, err, "adapter.json must be created")
}

func TestWriteInfo_WritesValidJSON(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewInfo("claude-code", "example/image:tag",
		map[string]any{"model": "gpt-5.4", "interactive": true},
		map[string]bool{"model": false, "interactive": true},
	)

	require.NoError(t, WriteInfo(dir, info))

	data, err := os.ReadFile(filepath.Join(dir, "adapter.json"))
	require.NoError(t, err)

	var parsed Info
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, info.SchemaVersion, parsed.SchemaVersion)
	require.Equal(t, info.Adapter, parsed.Adapter)
	require.Equal(t, info.Image, parsed.Image)
}

func TestWriteInfo_JSONMatchesSpecShape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	info := NewInfo("claude-code", "example/image:tag",
		map[string]any{"model": "gpt-5.4", "interactive": true},
		map[string]bool{"model": false, "interactive": true},
	)

	require.NoError(t, WriteInfo(dir, info))

	data, err := os.ReadFile(filepath.Join(dir, "adapter.json"))
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	var schemaVersion int
	require.NoError(t, json.Unmarshal(raw["schema_version"], &schemaVersion))
	require.Equal(t, 1, schemaVersion)

	var adapter string
	require.NoError(t, json.Unmarshal(raw["adapter"], &adapter))
	require.Equal(t, "claude-code", adapter)

	var image string
	require.NoError(t, json.Unmarshal(raw["image"], &image))
	require.Equal(t, "example/image:tag", image)

	var requested map[string]any
	require.NoError(t, json.Unmarshal(raw["requested"], &requested))
	require.Equal(t, "gpt-5.4", requested["model"])
	require.Equal(t, true, requested["interactive"])

	var applied map[string]bool
	require.NoError(t, json.Unmarshal(raw["applied"], &applied))
	require.False(t, applied["model"])
	require.True(t, applied["interactive"])
}
