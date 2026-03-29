//go:build integration

package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, out)
	}
}

func TestIsClean_Integration_CleanRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitInit(t, dir)

	runGit(t, dir, "commit", "--allow-empty", "-m", "initial")

	err := IsClean(context.Background(), dir)
	require.NoError(t, err)
}

func TestIsClean_Integration_StagedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitInit(t, dir)

	runGit(t, dir, "commit", "--allow-empty", "-m", "initial")

	f := filepath.Join(dir, "staged.txt")
	require.NoError(t, os.WriteFile(f, []byte("content"), 0o644))
	runGit(t, dir, "add", "staged.txt")

	err := IsClean(context.Background(), dir)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDirtyRepo)
}

func TestIsClean_Integration_UnstagedModifications(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitInit(t, dir)

	f := filepath.Join(dir, "tracked.txt")
	require.NoError(t, os.WriteFile(f, []byte("original"), 0o644))
	runGit(t, dir, "add", "tracked.txt")
	runGit(t, dir, "commit", "-m", "initial")

	require.NoError(t, os.WriteFile(f, []byte("modified"), 0o644))

	err := IsClean(context.Background(), dir)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDirtyRepo)
}

func TestIsClean_Integration_UntrackedNonIgnoredFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitInit(t, dir)

	runGit(t, dir, "commit", "--allow-empty", "-m", "initial")

	f := filepath.Join(dir, "untracked.txt")
	require.NoError(t, os.WriteFile(f, []byte("new"), 0o644))

	err := IsClean(context.Background(), dir)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDirtyRepo)
}

func TestIsClean_Integration_GitignoredFilesDoNotTrigger(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitInit(t, dir)

	runGit(t, dir, "commit", "--allow-empty", "-m", "initial")

	gitignore := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignore, []byte("*.log\n"), 0o644))
	runGit(t, dir, "add", ".gitignore")
	runGit(t, dir, "commit", "-m", "add gitignore")

	logFile := filepath.Join(dir, "debug.log")
	require.NoError(t, os.WriteFile(logFile, []byte("ignored"), 0o644))

	err := IsClean(context.Background(), dir)
	require.NoError(t, err)
}

func TestIsClean_Integration_EmptyRepoNoCommits(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitInit(t, dir)

	err := IsClean(context.Background(), dir)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDirtyRepo)
	require.ErrorContains(t, err, "no commits")
}

func TestIsClean_Integration_DeletedTrackedFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitInit(t, dir)

	f := filepath.Join(dir, "to-delete.txt")
	require.NoError(t, os.WriteFile(f, []byte("bye"), 0o644))
	runGit(t, dir, "add", "to-delete.txt")
	runGit(t, dir, "commit", "-m", "add file")

	require.NoError(t, os.Remove(f))

	err := IsClean(context.Background(), dir)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDirtyRepo)
}

func TestIsClean_Integration_NotAGitRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := IsClean(context.Background(), dir)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrDirtyRepo)
	require.ErrorContains(t, err, "not a git repository")
}
