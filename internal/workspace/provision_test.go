package workspace

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tessariq/tessariq/internal/container"
)

func TestBuildRepairArgs_ImagePinnedByDigest(t *testing.T) {
	t.Parallel()
	args := buildRepairArgs("/tmp/fakepath")

	// Find image: the element after the -v mount value and before "sh".
	shIdx := indexOf(args, "sh")
	require.GreaterOrEqual(t, shIdx, 1, "sh must be present")
	image := args[shIdx-1]
	require.True(t, strings.Contains(image, "@sha256:"),
		"repair image must be pinned by digest, got %q", image)
	require.Equal(t, container.RepairImage, image)
}

func TestBuildRepairArgs_SingleVolumeMount(t *testing.T) {
	t.Parallel()
	args := buildRepairArgs("/workspace/test-path")

	vFlags := collectAfter(args, "-v")
	require.Len(t, vFlags, 1, "exactly one -v mount expected")
	require.Equal(t, "/workspace/test-path:/work", vFlags[0])
}

func TestBuildRepairArgs_RunsAsRoot(t *testing.T) {
	t.Parallel()
	args := buildRepairArgs("/tmp/fakepath")

	userIdx := indexOf(args, "--user")
	require.GreaterOrEqual(t, userIdx, 0, "--user must be present")
	require.Equal(t, "root", args[userIdx+1])
}

func TestBuildRepairArgs_RemoveFlag(t *testing.T) {
	t.Parallel()
	args := buildRepairArgs("/tmp/fakepath")
	require.Contains(t, args, "--rm")
}

func TestBuildRepairArgs_ChownCommand(t *testing.T) {
	t.Parallel()
	args := buildRepairArgs("/tmp/fakepath")

	// Last arg is the shell command passed to "sh -c".
	fixCmd := args[len(args)-1]
	uid := os.Getuid()
	gid := os.Getgid()
	expected := fmt.Sprintf("chown -R %d:%d /work && chmod -R u+rwX /work", uid, gid)
	require.Equal(t, expected, fixCmd)
}

func TestBuildRepairArgs_CommandStructure(t *testing.T) {
	t.Parallel()
	args := buildRepairArgs("/tmp/fakepath")

	require.Equal(t, "run", args[0], "first arg must be 'run'")

	// Tail must be: [image, "sh", "-c", fixCmd]
	shIdx := indexOf(args, "sh")
	require.GreaterOrEqual(t, shIdx, 1)
	require.Equal(t, "-c", args[shIdx+1])
	require.Equal(t, container.RepairImage, args[shIdx-1], "image must precede sh")
}

func TestProvision_ParentDirMode_IsOwnerOnly(t *testing.T) {
	t.Parallel()
	// Provision will fail when calling git.AddWorktree because repoRoot is not
	// a real git repo, but the parent-dir mkdir runs first. That is what we
	// verify here.
	homeDir := t.TempDir()
	repoRoot := t.TempDir()

	_, _ = Provision(t.Context(), homeDir, repoRoot, "run-abc", t.TempDir(), "deadbeef", container.RuntimeIdentity{UID: os.Getuid(), GID: os.Getgid()})

	parent := fmt.Sprintf("%s/.tessariq/worktrees", homeDir)
	info, err := os.Stat(parent)
	require.NoError(t, err, ".tessariq/worktrees must exist after Provision attempt")
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
		"worktrees parent dir must be owner-only (0700)")
}

// indexOf returns the first index of needle in args, or -1.
func indexOf(args []string, needle string) int {
	for i, a := range args {
		if a == needle {
			return i
		}
	}
	return -1
}

// collectAfter returns all values that follow the given flag in args.
func collectAfter(args []string, flag string) []string {
	var result []string
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			result = append(result, args[i+1])
		}
	}
	return result
}
