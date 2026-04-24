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

// Validate checks that the agent info has a supported schema version and
// all spec-required fields are present.
func (a AgentInfo) Validate() error {
	if a.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", a.SchemaVersion)
	}
	if a.Agent == "" {
		return fmt.Errorf("missing required field %q", "agent")
	}
	return nil
}

// ReadAgentInfo reads and parses agent.json from the evidence directory.
func ReadAgentInfo(evidenceDir string) (AgentInfo, error) {
	data, err := os.ReadFile(filepath.Join(evidenceDir, "agent.json"))
	if err != nil {
		return AgentInfo{}, fmt.Errorf("read agent info: %w", err)
	}
	var info AgentInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return AgentInfo{}, fmt.Errorf("parse agent info: %w", err)
	}
	return info, nil
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
