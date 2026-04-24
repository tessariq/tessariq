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

// Validate checks that the manifest has a supported schema version and
// all spec-required fields are present.
func (m Manifest) Validate() error {
	if m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", m.SchemaVersion)
	}
	for _, check := range []struct{ field, value string }{
		{"run_id", m.RunID},
		{"task_path", m.TaskPath},
		{"agent", m.Agent},
		{"base_sha", m.BaseSHA},
		{"workspace_mode", m.WorkspaceMode},
		{"resolved_egress_mode", m.ResolvedEgressMode},
		{"container_name", m.ContainerName},
		{"created_at", m.CreatedAt},
	} {
		if check.value == "" {
			return fmt.Errorf("missing required field %q", check.field)
		}
	}
	return nil
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

	target := filepath.Join(dir, "manifest.json")
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write manifest temp file: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp) // best-effort cleanup
		return fmt.Errorf("rename manifest file: %w", err)
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
