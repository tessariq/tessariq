package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateEvidencePath_RejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	_, err := ValidateEvidencePath("/repo", "/tmp/evil")
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
	require.Contains(t, err.Error(), "outside the repository")
}

func TestValidateEvidencePath_RejectsParentTraversal(t *testing.T) {
	t.Parallel()

	_, err := ValidateEvidencePath("/repo", "../../outside")
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
}

func TestValidateEvidencePath_RejectsNestedTraversal(t *testing.T) {
	t.Parallel()

	_, err := ValidateEvidencePath("/repo", ".tessariq/runs/../../outside")
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
}

func TestValidateEvidencePath_RejectsPathOutsideRunsSubtree(t *testing.T) {
	t.Parallel()

	_, err := ValidateEvidencePath("/repo", ".tessariq/config")
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
}

func TestValidateEvidencePath_AcceptsCanonicalPath(t *testing.T) {
	t.Parallel()

	root := setupEvidenceRepo(t)
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tessariq", "runs", runID), 0o755))

	got, err := ValidateEvidencePath(root, filepath.Join(".tessariq", "runs", runID))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, ".tessariq", "runs", runID), got)
}

func TestValidateEvidencePath_RejectsEmptyPath(t *testing.T) {
	t.Parallel()

	_, err := ValidateEvidencePath("/repo", "")
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
}

func TestValidateEvidenceRunID_AcceptsMatchingRunID(t *testing.T) {
	t.Parallel()

	err := ValidateEvidenceRunID("/repo/.tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV", "01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.NoError(t, err)
}

func TestValidateEvidenceRunID_RejectsMismatchedRunID(t *testing.T) {
	t.Parallel()

	err := ValidateEvidenceRunID("/repo/.tessariq/runs/RUN_B", "RUN_A")
	require.ErrorIs(t, err, ErrEvidenceRunIDMismatch)
	require.Contains(t, err.Error(), "RUN_A")
	require.Contains(t, err.Error(), "RUN_B")
}

func setupEvidenceRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tessariq", "runs"), 0o755))
	realRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	return realRoot
}

func TestValidateEvidencePath_RejectsLeafSymlinkOutsideRepo(t *testing.T) {
	t.Parallel()

	root := setupEvidenceRepo(t)
	externalDir := t.TempDir()
	external, err := filepath.EvalSymlinks(externalDir)
	require.NoError(t, err)

	runID := "RUN_FORGED"
	require.NoError(t, os.MkdirAll(filepath.Join(external, runID), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(external, runID, "status.json"), []byte("{}"), 0o644))

	linkPath := filepath.Join(root, ".tessariq", "runs", runID)
	require.NoError(t, os.Symlink(filepath.Join(external, runID), linkPath))

	_, err = ValidateEvidencePath(root, filepath.Join(".tessariq", "runs", runID))
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
}

func TestValidateEvidencePath_RejectsIntermediateSymlinkOutsideRepo(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	root, err := filepath.EvalSymlinks(rootDir)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tessariq"), 0o755))

	externalDir := t.TempDir()
	external, err := filepath.EvalSymlinks(externalDir)
	require.NoError(t, err)

	runID := "RUN_INTERMEDIATE"
	require.NoError(t, os.MkdirAll(filepath.Join(external, "runs", runID), 0o755))

	// Symlink the intermediate `runs` directory outside the repo.
	require.NoError(t, os.Symlink(filepath.Join(external, "runs"), filepath.Join(root, ".tessariq", "runs")))

	_, err = ValidateEvidencePath(root, filepath.Join(".tessariq", "runs", runID))
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
}

func TestValidateEvidencePath_AcceptsIntraRepoSymlink(t *testing.T) {
	t.Parallel()

	root := setupEvidenceRepo(t)
	realRunID := "RUN_REAL"
	aliasID := "RUN_ALIAS"
	realPath := filepath.Join(root, ".tessariq", "runs", realRunID)
	require.NoError(t, os.MkdirAll(realPath, 0o755))

	aliasPath := filepath.Join(root, ".tessariq", "runs", aliasID)
	require.NoError(t, os.Symlink(realPath, aliasPath))

	got, err := ValidateEvidencePath(root, filepath.Join(".tessariq", "runs", aliasID))
	require.NoError(t, err)
	require.Equal(t, realPath, got)
}

func TestValidateEvidencePath_RejectsBrokenSymlink(t *testing.T) {
	t.Parallel()

	root := setupEvidenceRepo(t)
	runID := "RUN_BROKEN"
	linkPath := filepath.Join(root, ".tessariq", "runs", runID)
	require.NoError(t, os.Symlink("/nonexistent/evidence", linkPath))

	_, err := ValidateEvidencePath(root, filepath.Join(".tessariq", "runs", runID))
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
}

func TestValidateEvidencePath_RejectsNonexistentPath(t *testing.T) {
	t.Parallel()

	root := setupEvidenceRepo(t)
	_, err := ValidateEvidencePath(root, filepath.Join(".tessariq", "runs", "RUN_MISSING"))
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
}
