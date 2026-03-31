package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Metadata represents the workspace.json evidence artifact.
type Metadata struct {
	SchemaVersion   int    `json:"schema_version"`
	WorkspaceMode   string `json:"workspace_mode"`
	BaseSHA         string `json:"base_sha"`
	WorkspacePath   string `json:"workspace_path"`
	RepoMountMode   string `json:"repo_mount_mode"`
	RepoClean       bool   `json:"repo_clean"`
	Reproducibility string `json:"reproducibility"`
}

// BuildMetadata constructs workspace metadata with v0.1.0 worktree defaults.
func BuildMetadata(baseSHA, workspacePath string) Metadata {
	return Metadata{
		SchemaVersion:   1,
		WorkspaceMode:   "worktree",
		BaseSHA:         baseSHA,
		WorkspacePath:   workspacePath,
		RepoMountMode:   "rw",
		RepoClean:       true,
		Reproducibility: "strong",
	}
}

// WriteMetadata writes workspace.json into the given evidence directory.
func WriteMetadata(evidenceDir string, m Metadata) error {
	if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workspace metadata: %w", err)
	}

	path := filepath.Join(evidenceDir, "workspace.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write workspace metadata: %w", err)
	}

	return nil
}
