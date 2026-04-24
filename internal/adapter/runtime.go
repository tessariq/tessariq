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
//
// AuthMountMode records the host-side mount policy for all auth, config,
// and state paths. Disposable per-run runtime-state copies (used when an
// agent like Claude Code needs writable access to a state file) are an
// implementation detail of satisfying writes without weakening the host
// mount policy — they do not change AuthMountMode.
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
// authMountMode is the host-side mount policy string derived from the
// actual discovered mount specs; callers are expected to pass
// authmount.AuthMountModeReadOnly after having validated the contract
// via authmount.ValidateContract.
func NewRuntimeInfo(image, imageSource, authMountMode string, authMountCount int, agentConfigMount, agentConfigMountStatus string) RuntimeInfo {
	return RuntimeInfo{
		SchemaVersion:          1,
		Image:                  image,
		ImageSource:            imageSource,
		AuthMountMode:          authMountMode,
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

	target := filepath.Join(evidenceDir, "runtime.json")
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write runtime info temp file: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp) // best-effort cleanup
		return fmt.Errorf("rename runtime info file: %w", err)
	}

	return nil
}
