package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tessariq/tessariq/internal/run"
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

// proxyNonEmptyEvidenceFiles must be present and non-empty for proxy-mode runs.
var proxyNonEmptyEvidenceFiles = []string{
	"egress.compiled.yaml",
}

// proxyPresenceEvidenceFiles must be present for proxy-mode runs but may
// be 0 bytes (a valid run with no blocked egress events produces an empty
// events file). A missing file indicates telemetry extraction failed.
var proxyPresenceEvidenceFiles = []string{
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
		return fmt.Errorf("incomplete evidence: %w", err)
	}

	if manifest.ResolvedEgressMode == "proxy" {
		var proxyMissing []string
		proxyMissing = append(proxyMissing, collectMissing(evidenceDir, proxyNonEmptyEvidenceFiles)...)
		proxyMissing = append(proxyMissing, collectPresenceOnly(evidenceDir, proxyPresenceEvidenceFiles)...)
		if len(proxyMissing) > 0 {
			return incompleteErr(proxyMissing)
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

func collectPresenceOnly(evidenceDir string, names []string) []string {
	var missing []string
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(evidenceDir, name)); err != nil {
			missing = append(missing, name)
		}
	}
	return missing
}

func incompleteErr(missing []string) error {
	return fmt.Errorf("incomplete evidence: %s", strings.Join(missing, ", "))
}
