package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// CheckEvidenceCompleteness verifies that all required evidence files exist
// and are non-empty in the evidence directory.
func CheckEvidenceCompleteness(evidenceDir string) error {
	var missing []string
	for _, name := range requiredEvidenceFiles {
		info, err := os.Stat(filepath.Join(evidenceDir, name))
		if err != nil {
			missing = append(missing, name)
			continue
		}
		if info.Size() == 0 {
			missing = append(missing, name+" (empty)")
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("incomplete evidence: %s", strings.Join(missing, ", "))
	}
	return nil
}
