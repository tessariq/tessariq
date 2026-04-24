package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tessariq/tessariq/internal/proxy"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/workspace"
)

// ErrEgressModeMismatch is returned when manifest.json and runtime.json
// record different resolved egress modes, indicating evidence tampering.
var ErrEgressModeMismatch = errors.New("egress mode mismatch between manifest and runtime")

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

// proxyEvidenceFiles must be present and non-empty for proxy-mode runs.
// Zero-denied-events runs write a summary line so the file is non-empty.
// A missing or empty file indicates telemetry extraction failure.
var proxyEvidenceFiles = []string{
	"egress.compiled.yaml",
	"egress.events.jsonl",
}

// CheckEvidenceCompleteness verifies that all required evidence files exist
// and are non-empty in the evidence directory. Proxy-specific egress
// artifacts are required when the trusted resolved egress mode is "proxy".
//
// The trusted resolved egress mode is taken from runtime.json, which the
// host-side runner writes from the actual run assembly. The mode is required:
// a runtime.json that omits resolved_egress_mode fails closed rather than
// falling back to the mutable manifest. This prevents the suppression attack
// where a proxy run is relabeled to "direct" in manifest.json and the runtime
// field is dropped, which would otherwise skip the required proxy evidence.
// If runtime.json and manifest.json record different modes, the function
// returns ErrEgressModeMismatch — the evidence may have been tampered.
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

	runtimeMode, err := readRuntimeEgressMode(evidenceDir)
	if err != nil {
		return fmt.Errorf("incomplete evidence: %w", err)
	}
	if runtimeMode == "" {
		return fmt.Errorf("incomplete evidence: runtime.json missing resolved_egress_mode; cannot determine trusted egress mode")
	}

	if manifest.ResolvedEgressMode != runtimeMode {
		return fmt.Errorf("%w: manifest says %q but runtime says %q; evidence may be inconsistent or tampered",
			ErrEgressModeMismatch, manifest.ResolvedEgressMode, runtimeMode)
	}

	if runtimeMode == "proxy" {
		if missing := collectMissing(evidenceDir, proxyEvidenceFiles); len(missing) > 0 {
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

type runtimeEgressSnippet struct {
	ResolvedEgressMode string `json:"resolved_egress_mode"`
}

func readRuntimeEgressMode(evidenceDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(evidenceDir, "runtime.json"))
	if err != nil {
		return "", fmt.Errorf("read runtime: %w", err)
	}
	var s runtimeEgressSnippet
	if err := json.Unmarshal(data, &s); err != nil {
		return "", fmt.Errorf("parse runtime: %w", err)
	}
	return s.ResolvedEgressMode, nil
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
// its package (avoids runner→adapter import cycle). schema_version must equal
// the integer 1, and each field in requiredStringFields must be present as a
// non-empty string.
func validateStructuredJSON(evidenceDir, filename string, requiredStringFields []string) error {
	data, err := os.ReadFile(filepath.Join(evidenceDir, filename))
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parse %s: %w", filename, err)
	}
	sv, ok := m["schema_version"].(float64)
	if !ok || sv != 1 {
		return fmt.Errorf("unsupported schema_version %v", m["schema_version"])
	}
	for _, field := range requiredStringFields {
		v, ok := m[field]
		if !ok {
			return fmt.Errorf("missing required field %q", field)
		}
		s, isStr := v.(string)
		if !isStr || s == "" {
			return fmt.Errorf("missing required field %q", field)
		}
	}
	return nil
}
