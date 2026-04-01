package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// initTestGitRepo creates a minimal git repo in a temp dir with one commit.
func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	commands := [][]string{
		{"git", "init", "-b", "main", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "config", "commit.gpgsign", "false"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git command %v failed: %s", args, out)
	}
	return dir
}

func TestWriteDiffArtifacts_WithChanges(t *testing.T) {
	t.Parallel()

	worktree := initTestGitRepo(t)
	evidenceDir := t.TempDir()

	// Get base SHA.
	cmd := exec.Command("git", "-C", worktree, "rev-parse", "HEAD")
	shaOut, err := cmd.Output()
	require.NoError(t, err)
	baseSHA := string(shaOut[:len(shaOut)-1])

	// Create a file change.
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "new.txt"), []byte("hello\n"), 0o644))

	err = WriteDiffArtifacts(context.Background(), evidenceDir, worktree, baseSHA)
	require.NoError(t, err)

	// Both files should exist.
	patchData, err := os.ReadFile(filepath.Join(evidenceDir, "diff.patch"))
	require.NoError(t, err)
	require.NotEmpty(t, patchData)
	require.Contains(t, string(patchData), "new.txt")

	statData, err := os.ReadFile(filepath.Join(evidenceDir, "diffstat.txt"))
	require.NoError(t, err)
	require.NotEmpty(t, statData)
	require.Contains(t, string(statData), "new.txt")
}

func TestWriteDiffArtifacts_NoChanges(t *testing.T) {
	t.Parallel()

	worktree := initTestGitRepo(t)
	evidenceDir := t.TempDir()

	cmd := exec.Command("git", "-C", worktree, "rev-parse", "HEAD")
	shaOut, err := cmd.Output()
	require.NoError(t, err)
	baseSHA := string(shaOut[:len(shaOut)-1])

	err = WriteDiffArtifacts(context.Background(), evidenceDir, worktree, baseSHA)
	require.NoError(t, err)

	// Neither file should exist.
	_, err = os.Stat(filepath.Join(evidenceDir, "diff.patch"))
	require.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(evidenceDir, "diffstat.txt"))
	require.True(t, os.IsNotExist(err))
}

func TestWriteDiffArtifacts_FilePermissions(t *testing.T) {
	t.Parallel()

	worktree := initTestGitRepo(t)
	evidenceDir := t.TempDir()

	cmd := exec.Command("git", "-C", worktree, "rev-parse", "HEAD")
	shaOut, err := cmd.Output()
	require.NoError(t, err)
	baseSHA := string(shaOut[:len(shaOut)-1])

	require.NoError(t, os.WriteFile(filepath.Join(worktree, "file.txt"), []byte("data\n"), 0o644))

	err = WriteDiffArtifacts(context.Background(), evidenceDir, worktree, baseSHA)
	require.NoError(t, err)

	for _, name := range []string{"diff.patch", "diffstat.txt"} {
		info, err := os.Stat(filepath.Join(evidenceDir, name))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "%s permissions", name)
	}
}
