package run

import (
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

	got, err := ValidateEvidencePath("/repo", ".tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.NoError(t, err)
	require.Equal(t, filepath.Join("/repo", ".tessariq", "runs", "01ARZ3NDEKTSV4RRFFQ69G5FAV"), got)
}

func TestValidateEvidencePath_RejectsEmptyPath(t *testing.T) {
	t.Parallel()

	_, err := ValidateEvidencePath("/repo", "")
	require.ErrorIs(t, err, ErrEvidencePathOutsideRepo)
}
