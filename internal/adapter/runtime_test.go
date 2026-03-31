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

	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", 0, "disabled", "disabled")

	require.Equal(t, 1, info.SchemaVersion)
	require.Equal(t, "ghcr.io/tessariq/reference-runtime:v0.1.0", info.Image)
	require.Equal(t, "reference", info.ImageSource)
	require.Equal(t, "read-only", info.AuthMountMode)
	require.Equal(t, "disabled", info.AgentConfigMount)
	require.Equal(t, "disabled", info.AgentConfigMountStatus)
}

func TestNewRuntimeInfo_ReferenceImage(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", 0, "disabled", "disabled")

	require.Equal(t, "reference", info.ImageSource)
}

func TestNewRuntimeInfo_CustomImage(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("my-registry/custom:v1", "custom", 0, "disabled", "disabled")

	require.Equal(t, "custom", info.ImageSource)
}

func TestNewRuntimeInfo_ConfigMountMounted(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", 0, "enabled", "mounted")

	require.Equal(t, "enabled", info.AgentConfigMount)
	require.Equal(t, "mounted", info.AgentConfigMountStatus)
}

func TestNewRuntimeInfo_ConfigMountMissingOptional(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", 0, "enabled", "missing_optional")

	require.Equal(t, "enabled", info.AgentConfigMount)
	require.Equal(t, "missing_optional", info.AgentConfigMountStatus)
}

func TestNewRuntimeInfo_ConfigMountUnreadableOptional(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/claude-code:latest", "reference", 0, "enabled", "unreadable_optional")

	require.Equal(t, "enabled", info.AgentConfigMount)
	require.Equal(t, "unreadable_optional", info.AgentConfigMountStatus)
}

func TestNewRuntimeInfo_ExactlySevenTopLevelKeys(t *testing.T) {
	t.Parallel()

	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", 0, "disabled", "disabled")

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

func TestWriteRuntimeInfo_CreatesDirectoryAndFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", 0, "disabled", "disabled")

	require.NoError(t, WriteRuntimeInfo(dir, info))

	_, err := os.Stat(dir)
	require.NoError(t, err, "evidence directory must be created")

	_, err = os.Stat(filepath.Join(dir, "runtime.json"))
	require.NoError(t, err, "runtime.json must be created")
}

func TestWriteRuntimeInfo_WritesValidJSON(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", 0, "disabled", "disabled")

	require.NoError(t, WriteRuntimeInfo(dir, info))

	data, err := os.ReadFile(filepath.Join(dir, "runtime.json"))
	require.NoError(t, err)

	var parsed RuntimeInfo
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, info, parsed)
}

func TestWriteRuntimeInfo_JSONMatchesSpecShape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	info := NewRuntimeInfo("ghcr.io/tessariq/reference-runtime:v0.1.0", "reference", 0, "disabled", "disabled")

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
