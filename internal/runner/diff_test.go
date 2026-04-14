package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestWriteDiffArtifacts_SecondWriteFailure_NoOrphanDiffPatch asserts that a
// failure while committing the second diff artifact leaves no orphan diff.patch
// behind. It simulates the failure by pre-creating diffstat.txt as a directory
// so the writer cannot place a regular file at that path.
func TestWriteDiffArtifacts_SecondWriteFailure_NoOrphanDiffPatch(t *testing.T) {
	t.Parallel()

	worktree := initTestGitRepo(t)
	evidenceDir := t.TempDir()

	cmd := exec.Command("git", "-C", worktree, "rev-parse", "HEAD")
	shaOut, err := cmd.Output()
	require.NoError(t, err)
	baseSHA := strings.TrimSpace(string(shaOut))

	require.NoError(t, os.WriteFile(filepath.Join(worktree, "file.txt"), []byte("data\n"), 0o644))

	// Block the diffstat.txt path so the second commit step fails.
	require.NoError(t, os.Mkdir(filepath.Join(evidenceDir, "diffstat.txt"), 0o700))

	err = WriteDiffArtifacts(context.Background(), evidenceDir, worktree, baseSHA)
	require.Error(t, err, "writer must return an error when the second artifact cannot be written")

	// diff.patch must not be left behind as an orphan.
	_, statErr := os.Stat(filepath.Join(evidenceDir, "diff.patch"))
	require.True(t, os.IsNotExist(statErr),
		"diff.patch should not exist after second-write failure, got err=%v", statErr)

	// No leftover .tmp files.
	entries, err := os.ReadDir(evidenceDir)
	require.NoError(t, err)
	for _, e := range entries {
		require.False(t, strings.HasSuffix(e.Name(), ".tmp"),
			"unexpected leftover .tmp file: %s", e.Name())
	}
}

// TestWriteDiffArtifacts_SuccessLeavesNoTempFiles asserts that the successful
// path does not leave any .tmp artifacts in the evidence directory.
func TestWriteDiffArtifacts_SuccessLeavesNoTempFiles(t *testing.T) {
	t.Parallel()

	worktree := initTestGitRepo(t)
	evidenceDir := t.TempDir()

	cmd := exec.Command("git", "-C", worktree, "rev-parse", "HEAD")
	shaOut, err := cmd.Output()
	require.NoError(t, err)
	baseSHA := strings.TrimSpace(string(shaOut))

	require.NoError(t, os.WriteFile(filepath.Join(worktree, "tmpcheck.txt"), []byte("x\n"), 0o644))

	require.NoError(t, WriteDiffArtifacts(context.Background(), evidenceDir, worktree, baseSHA))

	entries, err := os.ReadDir(evidenceDir)
	require.NoError(t, err)
	for _, e := range entries {
		require.False(t, strings.HasSuffix(e.Name(), ".tmp"),
			"unexpected leftover .tmp file: %s", e.Name())
	}
}
