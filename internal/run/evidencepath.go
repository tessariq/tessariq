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

// ErrEvidenceRunIDMismatch is returned when the evidence directory name
// does not match the expected run ID.
var ErrEvidenceRunIDMismatch = errors.New("evidence run_id mismatch")

// ValidateEvidencePath checks that evidencePath is a relative path whose
// real filesystem target is strictly within <repoRoot>/.tessariq/runs/.
// Symlinks in the evidence path or its ancestors are resolved before the
// containment check so a symlink planted under .tessariq/runs/ cannot be
// used to escape the repository. It returns the resolved absolute path on
// success.
func ValidateEvidencePath(repoRoot, evidencePath string) (string, error) {
	if evidencePath == "" || filepath.IsAbs(evidencePath) {
		return "", fmt.Errorf("%w: %s", ErrEvidencePathOutsideRepo, evidencePath)
	}

	cleanRoot := filepath.Clean(repoRoot)
	absPath := filepath.Clean(filepath.Join(cleanRoot, evidencePath))
	lexicalRunsPrefix := filepath.Join(cleanRoot, ".tessariq", "runs") + string(filepath.Separator)
	if !strings.HasPrefix(absPath+string(filepath.Separator), lexicalRunsPrefix) {
		return "", fmt.Errorf("%w: %s", ErrEvidencePathOutsideRepo, evidencePath)
	}

	realRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrEvidencePathOutsideRepo, evidencePath)
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrEvidencePathOutsideRepo, evidencePath)
	}

	realRunsPrefix := filepath.Join(realRoot, ".tessariq", "runs") + string(filepath.Separator)
	if !strings.HasPrefix(realPath+string(filepath.Separator), realRunsPrefix) {
		return "", fmt.Errorf("%w: %s", ErrEvidencePathOutsideRepo, evidencePath)
	}

	return realPath, nil
}

// ValidateEvidenceRunID checks that the evidence directory name matches the
// expected run ID. evidenceDir must be the absolute path returned by
// ValidateEvidencePath.
func ValidateEvidenceRunID(evidenceDir, runID string) error {
	dirName := filepath.Base(evidenceDir)
	if dirName != runID {
		return fmt.Errorf("%w: evidence directory %s does not belong to run %s", ErrEvidenceRunIDMismatch, dirName, runID)
	}
	return nil
}
