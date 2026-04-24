package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tessariq/tessariq/internal/proxy"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/workspace"
)

// requiredEvidenceFiles lists the files that must be present and non-empty
// for every completed run.
var requiredEvidenceFiles = []string{
	"manifest.json",
	"status.json",
	"agent.json",
	"runtime.json",
	"task.md",
	"run.log",
	"runner.log",
	"workspace.json",
}

// proxyRequiredEvidenceFiles lists the additional files that must be present
// and non-empty when a run resolved its egress mode to "proxy".
var proxyRequiredEvidenceFiles = []string{
	"egress.compiled.yaml",
	"egress.events.jsonl",
}

// CheckEvidenceCompleteness verifies that all required evidence files exist
// and are non-empty in the evidence directory. For runs whose manifest
// records resolved_egress_mode=proxy, the proxy-specific egress artifacts
// are also required.
func CheckEvidenceCompleteness(evidenceDir string) error {
	if missing := collectMissing(evidenceDir, requiredEvidenceFiles); len(missing) > 0 {
		return incompleteErr(missing)
	}

	manifest, err := run.ReadManifest(evidenceDir)
	if err != nil {
		return malformedErr("manifest.json", err)
	}
	if err := manifest.Validate(); err != nil {
		return malformedErr("manifest.json", err)
	}

	status, err := ReadStatus(evidenceDir)
	if err != nil {
		return malformedErr("status.json", err)
	}
	if err := status.Validate(); err != nil {
		return malformedErr("status.json", err)
	}

	if err := validateStructuredJSON(evidenceDir, "agent.json", []string{"agent"}); err != nil {
		return malformedErr("agent.json", err)
	}

	if err := validateStructuredJSON(evidenceDir, "runtime.json", []string{
		"image", "image_source", "auth_mount_mode",
		"agent_config_mount", "agent_config_mount_status",
	}); err != nil {
		return malformedErr("runtime.json", err)
	}

	workspaceMeta, err := workspace.ReadMetadata(evidenceDir)
	if err != nil {
		return malformedErr("workspace.json", err)
	}
	if err := workspaceMeta.Validate(); err != nil {
		return malformedErr("workspace.json", err)
	}

	if manifest.ResolvedEgressMode == "proxy" {
		if missing := collectMissing(evidenceDir, proxyRequiredEvidenceFiles); len(missing) > 0 {
			return incompleteErr(missing)
		}

		compiled, err := proxy.ReadCompiledYAML(evidenceDir)
		if err != nil {
			return malformedErr("egress.compiled.yaml", err)
		}
		if err := compiled.Validate(); err != nil {
			return malformedErr("egress.compiled.yaml", err)
		}

		if _, err := proxy.ReadEventsJSONL(evidenceDir); err != nil {
			return malformedErr("egress.events.jsonl", err)
		}
	}

	return nil
}

func collectMissing(evidenceDir string, names []string) []string {
	var missing []string
	for _, name := range names {
		info, err := os.Stat(filepath.Join(evidenceDir, name))
		if err != nil {
			missing = append(missing, name)
			continue
		}
		if info.Size() == 0 {
			missing = append(missing, name+" (empty)")
		}
	}
	return missing
}

func incompleteErr(missing []string) error {
	return fmt.Errorf("incomplete evidence: %s", strings.Join(missing, ", "))
}

func malformedErr(artifact string, cause error) error {
	return fmt.Errorf("malformed evidence %s: %w", artifact, cause)
}

// validateStructuredJSON validates a JSON evidence artifact without importing
// its package (avoids runner→adapter import cycle). Each field in
// requiredStringFields must be present and, when its JSON value is a string,
// must be non-empty.
func validateStructuredJSON(evidenceDir, filename string, requiredStringFields []string) error {
	data, err := os.ReadFile(filepath.Join(evidenceDir, filename))
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parse %s: %w", filename, err)
	}
	sv, _ := m["schema_version"].(float64)
	if int(sv) != 1 {
		return fmt.Errorf("unsupported schema_version %v", m["schema_version"])
	}
	for _, field := range requiredStringFields {
		v, ok := m[field]
		if !ok {
			return fmt.Errorf("missing required field %q", field)
		}
		if s, isStr := v.(string); isStr && s == "" {
			return fmt.Errorf("missing required field %q", field)
		}
	}
	return nil
}
