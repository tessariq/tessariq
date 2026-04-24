package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AgentInfo represents the agent.json evidence artifact.
type AgentInfo struct {
	SchemaVersion int             `json:"schema_version"`
	Agent         string          `json:"agent"`
	Requested     map[string]any  `json:"requested"`
	Supported     map[string]bool `json:"supported"`
}

// NewAgentInfo creates an agent.json artifact with the given fields.
// Requested records raw user-provided option values; supported records
// whether the selected agent supports honoring each recorded option exactly.
func NewAgentInfo(agent string, requested map[string]any, supported map[string]bool) AgentInfo {
	return AgentInfo{
		SchemaVersion: 1,
		Agent:         agent,
		Requested:     requested,
		Supported:     supported,
	}
}

// WriteAgentInfo writes agent.json into the given evidence directory.
func WriteAgentInfo(evidenceDir string, info AgentInfo) error {
	if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agent info: %w", err)
	}

	target := filepath.Join(evidenceDir, "agent.json")
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write agent info temp file: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp) // best-effort cleanup
		return fmt.Errorf("rename agent info file: %w", err)
	}

	return nil
}
