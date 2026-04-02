package run

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrEvidencePathOutsideRepo is returned when an evidence path is not
// within the repository's .tessariq/runs/ directory.
var ErrEvidencePathOutsideRepo = errors.New("evidence path is outside the repository")

// ValidateEvidencePath checks that evidencePath is a relative path that
// resolves strictly within <repoRoot>/.tessariq/runs/. It returns the
// validated absolute path on success.
func ValidateEvidencePath(repoRoot, evidencePath string) (string, error) {
	if evidencePath == "" || filepath.IsAbs(evidencePath) {
		return "", fmt.Errorf("%w: %s", ErrEvidencePathOutsideRepo, evidencePath)
	}

	absPath := filepath.Clean(filepath.Join(repoRoot, evidencePath))
	runsPrefix := filepath.Join(filepath.Clean(repoRoot), ".tessariq", "runs") + string(filepath.Separator)

	if !strings.HasPrefix(absPath+string(filepath.Separator), runsPrefix) {
		return "", fmt.Errorf("%w: %s", ErrEvidencePathOutsideRepo, evidencePath)
	}

	return absPath, nil
}
