package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AgentUpdate records the outcome of the agent auto-update init container.
type AgentUpdate struct {
	Attempted     bool   `json:"attempted"`
	Success       bool   `json:"success"`
	CachedVersion string `json:"cached_version"`
	BakedVersion  string `json:"baked_version"`
	ElapsedMs     int64  `json:"elapsed_ms"`
	Error         string `json:"error"`
}

// RuntimeInfo represents the runtime.json evidence artifact.
type RuntimeInfo struct {
	SchemaVersion          int          `json:"schema_version"`
	Image                  string       `json:"image"`
	ImageSource            string       `json:"image_source"`
	AuthMountMode          string       `json:"auth_mount_mode"`
	AuthMountCount         int          `json:"auth_mount_count"`
	AgentConfigMount       string       `json:"agent_config_mount"`
	AgentConfigMountStatus string       `json:"agent_config_mount_status"`
	AgentUpdate            *AgentUpdate `json:"agent_update,omitempty"`
}

// NewRuntimeInfo creates a runtime.json artifact with the given fields.
// AuthMountMode is always "read-only" in v0.1.0.
func NewRuntimeInfo(image, imageSource string, authMountCount int, agentConfigMount, agentConfigMountStatus string) RuntimeInfo {
	return RuntimeInfo{
		SchemaVersion:          1,
		Image:                  image,
		ImageSource:            imageSource,
		AuthMountMode:          "read-only",
		AuthMountCount:         authMountCount,
		AgentConfigMount:       agentConfigMount,
		AgentConfigMountStatus: agentConfigMountStatus,
	}
}

// WriteRuntimeInfo writes runtime.json into the given evidence directory.
func WriteRuntimeInfo(evidenceDir string, info RuntimeInfo) error {
	if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runtime info: %w", err)
	}

	path := filepath.Join(evidenceDir, "runtime.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write runtime info: %w", err)
	}

	return nil
}
