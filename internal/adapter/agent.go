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
	Applied       map[string]bool `json:"applied"`
}

// NewAgentInfo creates an agent.json artifact with the given fields.
// Requested records raw user-provided option values; applied records
// whether the selected agent supports honoring each recorded option exactly.
func NewAgentInfo(agent string, requested map[string]any, applied map[string]bool) AgentInfo {
	return AgentInfo{
		SchemaVersion: 1,
		Agent:         agent,
		Requested:     requested,
		Applied:       applied,
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

	path := filepath.Join(evidenceDir, "agent.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write agent info: %w", err)
	}

	return nil
}
