package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Info represents the adapter.json evidence artifact.
type Info struct {
	SchemaVersion int             `json:"schema_version"`
	Adapter       string          `json:"adapter"`
	Image         string          `json:"image"`
	Requested     map[string]any  `json:"requested"`
	Applied       map[string]bool `json:"applied"`
}

// NewInfo creates an adapter.json artifact with the given fields.
// Requested records raw user-provided option values; applied records
// whether each requested option was successfully applied by the adapter.
func NewInfo(adapter, image string, requested map[string]any, applied map[string]bool) Info {
	return Info{
		SchemaVersion: 1,
		Adapter:       adapter,
		Image:         image,
		Requested:     requested,
		Applied:       applied,
	}
}

// WriteInfo writes adapter.json into the given evidence directory.
func WriteInfo(evidenceDir string, info Info) error {
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal adapter info: %w", err)
	}

	path := filepath.Join(evidenceDir, "adapter.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write adapter info: %w", err)
	}

	return nil
}
