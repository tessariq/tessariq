package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tessariq/tessariq/internal/run"
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

// proxyRequiredEvidenceFiles lists the additional files that must be present
// and non-empty when a run resolved its egress mode to "proxy".
var proxyRequiredEvidenceFiles = []string{
	"egress.compiled.yaml",
	"egress.events.jsonl",
}

// CheckEvidenceCompleteness verifies that all required evidence files exist
// and are non-empty in the evidence directory. Proxy-specific egress
// artifacts are required when the trusted resolved egress mode is "proxy".
//
// The trusted mode is derived by cross-checking manifest.json and
// runtime.json. If both record a mode and they disagree, the function
// returns ErrEgressModeMismatch — the evidence may have been tampered.
func CheckEvidenceCompleteness(evidenceDir string) error {
	if missing := collectMissing(evidenceDir, requiredEvidenceFiles); len(missing) > 0 {
		return incompleteErr(missing)
	}

	manifest, err := run.ReadManifest(evidenceDir)
	if err != nil {
		return fmt.Errorf("incomplete evidence: %w", err)
	}

	runtimeMode, err := readRuntimeEgressMode(evidenceDir)
	if err != nil {
		return fmt.Errorf("incomplete evidence: %w", err)
	}

	if runtimeMode != "" && manifest.ResolvedEgressMode != runtimeMode {
		return fmt.Errorf("%w: manifest says %q but runtime says %q; evidence may be inconsistent or tampered",
			ErrEgressModeMismatch, manifest.ResolvedEgressMode, runtimeMode)
	}

	// Fallback to manifest when runtime.json pre-dates this field (runs
	// created before TASK-098). The mismatch guard above already fires when
	// runtime carries a non-empty disagreeing value.
	trustedMode := runtimeMode
	if trustedMode == "" {
		trustedMode = manifest.ResolvedEgressMode
	}

	if trustedMode == "proxy" {
		if missing := collectMissing(evidenceDir, proxyRequiredEvidenceFiles); len(missing) > 0 {
			return incompleteErr(missing)
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
