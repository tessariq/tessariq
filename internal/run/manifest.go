package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Manifest struct {
	SchemaVersion       int    `json:"schema_version"`
	RunID               string `json:"run_id"`
	TaskPath            string `json:"task_path"`
	TaskTitle           string `json:"task_title"`
	Adapter             string `json:"adapter"`
	BaseSHA             string `json:"base_sha"`
	WorkspaceMode       string `json:"workspace_mode"`
	RequestedEgressMode string `json:"requested_egress_mode"`
	ContainerName       string `json:"container_name"`
	CreatedAt           string `json:"created_at"`
}

func BuildManifestSeed(cfg Config, runID, taskTitle, baseSHA string, now time.Time) Manifest {
	return Manifest{
		SchemaVersion:       1,
		RunID:               runID,
		TaskPath:            cfg.TaskPath,
		TaskTitle:           taskTitle,
		Adapter:             cfg.Agent,
		BaseSHA:             baseSHA,
		WorkspaceMode:       "worktree",
		RequestedEgressMode: cfg.ResolveEgress(),
		ContainerName:       ContainerName(runID),
		CreatedAt:           now.UTC().Format(time.RFC3339),
	}
}

func WriteManifest(dir string, m Manifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func BootstrapManifest(repoRoot string, cfg Config, taskTitle, baseSHA string, now time.Time) (string, string, error) {
	runID, err := NewRunID(now)
	if err != nil {
		return "", "", fmt.Errorf("generate run ID: %w", err)
	}

	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	m := BuildManifestSeed(cfg, runID, taskTitle, baseSHA, now)

	if err := WriteManifest(evidenceDir, m); err != nil {
		return "", "", fmt.Errorf("bootstrap manifest: %w", err)
	}

	return runID, evidenceDir, nil
}
