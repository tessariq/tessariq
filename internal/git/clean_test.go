package git

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePorcelain_CleanRepo(t *testing.T) {
	t.Parallel()

	err := parsePorcelain("")
	require.NoError(t, err)
}

func TestParsePorcelain_StagedFile(t *testing.T) {
	t.Parallel()

	err := parsePorcelain("M  file.txt")
	require.True(t, errors.Is(err, ErrDirtyRepo), "expected ErrDirtyRepo, got: %v", err)
}

func TestParsePorcelain_UnstagedModification(t *testing.T) {
	t.Parallel()

	err := parsePorcelain(" M file.txt")
	require.True(t, errors.Is(err, ErrDirtyRepo), "expected ErrDirtyRepo, got: %v", err)
}

func TestParsePorcelain_UntrackedFile(t *testing.T) {
	t.Parallel()

	err := parsePorcelain("?? newfile.txt")
	require.True(t, errors.Is(err, ErrDirtyRepo), "expected ErrDirtyRepo, got: %v", err)
}

func TestParsePorcelain_DeletedFile(t *testing.T) {
	t.Parallel()

	err := parsePorcelain("D  deleted.txt")
	require.True(t, errors.Is(err, ErrDirtyRepo), "expected ErrDirtyRepo, got: %v", err)
}

func TestParsePorcelain_RenamedFile(t *testing.T) {
	t.Parallel()

	err := parsePorcelain("R  old.txt -> new.txt")
	require.True(t, errors.Is(err, ErrDirtyRepo), "expected ErrDirtyRepo, got: %v", err)
}

func TestParsePorcelain_MultipleEntries(t *testing.T) {
	t.Parallel()

	output := "M  staged.txt\n?? untracked.txt\n M modified.txt"
	err := parsePorcelain(output)
	require.True(t, errors.Is(err, ErrDirtyRepo), "expected ErrDirtyRepo, got: %v", err)
}

func TestParsePorcelain_OnlyWhitespace(t *testing.T) {
	t.Parallel()

	err := parsePorcelain("   \n  \n")
	require.NoError(t, err)
}

func TestParsePorcelain_UntrackedAndIgnoredLines(t *testing.T) {
	t.Parallel()

	err := parsePorcelain("?? tracked.txt\n!! actually_ignored.log")
	require.True(t, errors.Is(err, ErrDirtyRepo), "expected ErrDirtyRepo, got: %v", err)
}

func TestParsePorcelain_ErrorMessageFormat(t *testing.T) {
	t.Parallel()

	err := parsePorcelain("?? newfile.txt")
	require.ErrorContains(t, err, "commit, stash, or clean")
}

func TestParsePorcelain_ShortStatus(t *testing.T) {
	t.Parallel()

	err := parsePorcelain("AM newfile.txt")
	require.True(t, errors.Is(err, ErrDirtyRepo), "expected ErrDirtyRepo, got: %v", err)
}
