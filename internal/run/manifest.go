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
	Agent               string `json:"agent"`
	BaseSHA             string `json:"base_sha"`
	WorkspaceMode       string `json:"workspace_mode"`
	RequestedEgressMode string `json:"requested_egress_mode"`
	ResolvedEgressMode  string `json:"resolved_egress_mode"`
	AllowlistSource     string `json:"allowlist_source"`
	ContainerName       string `json:"container_name"`
	CreatedAt           string `json:"created_at"`
}

func BuildManifestSeed(cfg Config, runID, taskTitle, baseSHA, allowlistSource string, now time.Time) Manifest {
	requestedEgress := cfg.ResolveEgress()
	resolvedEgress := ResolveEgressMode(requestedEgress)

	return Manifest{
		SchemaVersion:       1,
		RunID:               runID,
		TaskPath:            cfg.TaskPath,
		TaskTitle:           taskTitle,
		Agent:               cfg.Agent,
		BaseSHA:             baseSHA,
		WorkspaceMode:       "worktree",
		RequestedEgressMode: requestedEgress,
		ResolvedEgressMode:  resolvedEgress,
		AllowlistSource:     allowlistSource,
		ContainerName:       ContainerName(runID),
		CreatedAt:           now.UTC().Format(time.RFC3339),
	}
}

// ResolveEgressMode maps the requested egress mode to the actual mode.
// In v0.1.0, "auto" resolves to "proxy" for all agents.
func ResolveEgressMode(requested string) string {
	if requested == "auto" {
		return "proxy"
	}
	return requested
}

// ReadManifest reads the manifest from the evidence directory.
func ReadManifest(evidenceDir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(evidenceDir, "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}

	return m, nil
}

func WriteManifest(dir string, m Manifest) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func BootstrapManifest(repoRoot string, cfg Config, taskTitle, baseSHA, allowlistSource string, now time.Time) (string, string, error) {
	runID, err := NewRunID(now)
	if err != nil {
		return "", "", fmt.Errorf("generate run ID: %w", err)
	}

	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	m := BuildManifestSeed(cfg, runID, taskTitle, baseSHA, allowlistSource, now)

	if err := WriteManifest(evidenceDir, m); err != nil {
		return "", "", fmt.Errorf("bootstrap manifest: %w", err)
	}

	return runID, evidenceDir, nil
}
