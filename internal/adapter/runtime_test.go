package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRuntimeInfo_RequiredFields(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", "read-only", 0, "disabled", "disabled")

	require.Equal(t, 1, info.SchemaVersion)
	require.Equal(t, "ghcr.io/tessariq/reference-runtime:v0.1.0", info.Image)
	require.Equal(t, "reference", info.ImageSource)
	require.Equal(t, "read-only", info.AuthMountMode)
	require.Equal(t, "disabled", info.AgentConfigMount)
	require.Equal(t, "disabled", info.AgentConfigMountStatus)
}

func TestNewRuntimeInfo_ReferenceImage(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", "read-only", 0, "disabled", "disabled")

	require.Equal(t, "reference", info.ImageSource)
}

func TestNewRuntimeInfo_CustomImage(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("my-registry/custom:v1", "custom", "read-only", 0, "disabled", "disabled")

	require.Equal(t, "custom", info.ImageSource)
}

func TestNewRuntimeInfo_ConfigMountMounted(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", "read-only", 0, "enabled", "mounted")

	require.Equal(t, "enabled", info.AgentConfigMount)
	require.Equal(t, "mounted", info.AgentConfigMountStatus)
}

func TestNewRuntimeInfo_ConfigMountMissingOptional(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", "read-only", 0, "enabled", "missing_optional")

	require.Equal(t, "enabled", info.AgentConfigMount)
	require.Equal(t, "missing_optional", info.AgentConfigMountStatus)
}

func TestNewRuntimeInfo_ConfigMountUnreadableOptional(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", "read-only", 0, "enabled", "unreadable_optional")

	require.Equal(t, "enabled", info.AgentConfigMount)
	require.Equal(t, "unreadable_optional", info.AgentConfigMountStatus)
}

func TestNewRuntimeInfo_ExactlySevenTopLevelKeys(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", "read-only", 0, "disabled", "disabled")

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	expectedKeys := map[string]bool{
		"schema_version":            true,
		"image":                     true,
		"image_source":              true,
		"auth_mount_mode":           true,
		"auth_mount_count":          true,
		"agent_config_mount":        true,
		"agent_config_mount_status": true,
	}

	for k := range raw {
		require.True(t, expectedKeys[k], "unexpected key: %s", k)
	}
	require.Len(t, raw, len(expectedKeys))
}

func TestWriteRuntimeInfo_DirectoryPermissions(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", "read-only", 0, "disabled", "disabled")

	require.NoError(t, WriteRuntimeInfo(dir, info))

	stat, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), stat.Mode().Perm(),
		"evidence directory must be owner-only")
}

func TestWriteRuntimeInfo_FilePermissions(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", "read-only", 0, "disabled", "disabled")

	require.NoError(t, WriteRuntimeInfo(dir, info))

	stat, err := os.Stat(filepath.Join(dir, "runtime.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), stat.Mode().Perm(),
		"evidence file must be owner-only read/write")
}

func TestWriteRuntimeInfo_CreatesDirectoryAndFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", "read-only", 0, "disabled", "disabled")

	require.NoError(t, WriteRuntimeInfo(dir, info))

	_, err := os.Stat(dir)
	require.NoError(t, err, "evidence directory must be created")

	_, err = os.Stat(filepath.Join(dir, "runtime.json"))
	require.NoError(t, err, "runtime.json must be created")
}

func TestWriteRuntimeInfo_WritesValidJSON(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", "read-only", 0, "disabled", "disabled")

	require.NoError(t, WriteRuntimeInfo(dir, info))

	data, err := os.ReadFile(filepath.Join(dir, "runtime.json"))
	require.NoError(t, err)

	var parsed RuntimeInfo
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, info, parsed)
}

func TestAgentUpdate_SuccessJSON(t *testing.T) {
	t.Parallel()

	update := AgentUpdate{
		Attempted:     true,
		Success:       true,
		CachedVersion: "2.3.0",
		BakedVersion:  "2.1.92",
		ElapsedMs:     4200,
		Error:         "",
	}

	data, err := json.Marshal(update)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	var attempted bool
	require.NoError(t, json.Unmarshal(raw["attempted"], &attempted))
	require.True(t, attempted)

	var success bool
	require.NoError(t, json.Unmarshal(raw["success"], &success))
	require.True(t, success)

	var cachedVersion string
	require.NoError(t, json.Unmarshal(raw["cached_version"], &cachedVersion))
	require.Equal(t, "2.3.0", cachedVersion)

	var bakedVersion string
	require.NoError(t, json.Unmarshal(raw["baked_version"], &bakedVersion))
	require.Equal(t, "2.1.92", bakedVersion)

	var elapsedMs int64
	require.NoError(t, json.Unmarshal(raw["elapsed_ms"], &elapsedMs))
	require.Equal(t, int64(4200), elapsedMs)
}

func TestAgentUpdate_FailureJSON(t *testing.T) {
	t.Parallel()

	update := AgentUpdate{
		Attempted:    true,
		Success:      false,
		BakedVersion: "2.1.92",
		ElapsedMs:    1500,
		Error:        "npm ERR! network timeout",
	}

	data, err := json.Marshal(update)
	require.NoError(t, err)

	var parsed AgentUpdate
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.False(t, parsed.Success)
	require.Equal(t, "npm ERR! network timeout", parsed.Error)
	require.Empty(t, parsed.CachedVersion)
}

func TestAgentUpdate_SkippedJSON(t *testing.T) {
	t.Parallel()

	update := AgentUpdate{
		Attempted: false,
	}

	data, err := json.Marshal(update)
	require.NoError(t, err)

	var parsed AgentUpdate
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.False(t, parsed.Attempted)
	require.False(t, parsed.Success)
}

func TestRuntimeInfo_OmitsAgentUpdateWhenNil(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", "read-only", 0, "disabled", "disabled")

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	_, hasUpdate := raw["agent_update"]
	require.False(t, hasUpdate, "agent_update must be omitted when nil")
}

func TestRuntimeInfo_IncludesAgentUpdateWhenSet(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", "read-only", 0, "disabled", "disabled")
	info.AgentUpdate = &AgentUpdate{
		Attempted:     true,
		Success:       true,
		CachedVersion: "2.3.0",
		BakedVersion:  "2.1.92",
		ElapsedMs:     4200,
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Len(t, raw, 8, "runtime.json must have 8 top-level keys when agent_update is set")
	_, hasUpdate := raw["agent_update"]
	require.True(t, hasUpdate, "agent_update must be present")
}

func TestWriteRuntimeInfo_WithAgentUpdate(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", "read-only", 0, "disabled", "disabled")
	info.AgentUpdate = &AgentUpdate{
		Attempted:     true,
		Success:       true,
		CachedVersion: "2.3.0",
		BakedVersion:  "2.1.92",
		ElapsedMs:     4200,
	}

	require.NoError(t, WriteRuntimeInfo(dir, info))

	data, err := os.ReadFile(filepath.Join(dir, "runtime.json"))
	require.NoError(t, err)

	var parsed RuntimeInfo
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.NotNil(t, parsed.AgentUpdate)
	require.True(t, parsed.AgentUpdate.Attempted)
	require.True(t, parsed.AgentUpdate.Success)
	require.Equal(t, "2.3.0", parsed.AgentUpdate.CachedVersion)
	require.Equal(t, "2.1.92", parsed.AgentUpdate.BakedVersion)
	require.Equal(t, int64(4200), parsed.AgentUpdate.ElapsedMs)
}

func TestWriteRuntimeInfo_JSONMatchesSpecShape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", "read-only", 0, "disabled", "disabled")

	require.NoError(t, WriteRuntimeInfo(dir, info))

	data, err := os.ReadFile(filepath.Join(dir, "runtime.json"))
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	var schemaVersion int
	require.NoError(t, json.Unmarshal(raw["schema_version"], &schemaVersion))
	require.Equal(t, 1, schemaVersion)

	var image string
	require.NoError(t, json.Unmarshal(raw["image"], &image))
	require.Equal(t, "ghcr.io/tessariq/reference-runtime:v0.1.0", image)

	var imageSource string
	require.NoError(t, json.Unmarshal(raw["image_source"], &imageSource))
	require.Equal(t, "reference", imageSource)

	var authMountMode string
	require.NoError(t, json.Unmarshal(raw["auth_mount_mode"], &authMountMode))
	require.Equal(t, "read-only", authMountMode)

	var agentConfigMount string
	require.NoError(t, json.Unmarshal(raw["agent_config_mount"], &agentConfigMount))
	require.Equal(t, "disabled", agentConfigMount)

	var agentConfigMountStatus string
	require.NoError(t, json.Unmarshal(raw["agent_config_mount_status"], &agentConfigMountStatus))
	require.Equal(t, "disabled", agentConfigMountStatus)
}
