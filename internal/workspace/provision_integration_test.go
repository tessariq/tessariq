//go:build integration

package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

var testRuntimeIdentity = container.RuntimeIdentity{UID: 1000, GID: 1000}

func TestWorkspacePath_Layout(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	repoRoot := "/home/user/code/tessariq"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	p := WorkspacePath(homeDir, repoRoot, runID)

	require.Contains(t, p, filepath.Join(homeDir, ".tessariq", "worktrees"))
	require.Contains(t, p, runID)
	require.Contains(t, p, RepoID(repoRoot))
}

func resolveHead(t *testing.T, ctx context.Context, repo *containers.GitRepo) string {
	t.Helper()
	sha, err := repo.ExecOutput(ctx, "rev-parse", "HEAD")
	require.NoError(t, err)
	return sha
}

func TestProvision_Integration_CreatesWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), runID, evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	// Worktree directory exists
	info, err := os.Stat(wsPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestProvision_Integration_WritesWorkspaceJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), runID, evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(evidenceDir, "workspace.json"))
	require.NoError(t, err)

	var m Metadata
	require.NoError(t, json.Unmarshal(data, &m))

	require.Equal(t, 1, m.SchemaVersion)
	require.Equal(t, "worktree", m.WorkspaceMode)
	require.Equal(t, headSHA, m.BaseSHA)
	require.Equal(t, wsPath, m.WorkspacePath)
	require.Equal(t, "rw", m.RepoMountMode)
	require.True(t, m.RepoClean)
	require.Equal(t, "strong", m.Reproducibility)
}

func TestProvision_Integration_WorktreeCheckedOutAtProvidedSHA(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	// Verify the worktree is checked out at the provided SHA.
	out, err := exec.CommandContext(ctx, "git", "-C", wsPath, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	require.Equal(t, headSHA, strings.TrimSpace(string(out)))
}

func TestProvision_Integration_UsesCallerProvidedSHA(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	// Capture the first commit SHA.
	firstSHA := resolveHead(t, ctx, repo)

	// Advance HEAD with a second commit so HEAD != firstSHA.
	_, err = repo.ExecOutput(ctx, "commit", "--allow-empty", "-m", "second commit")
	require.NoError(t, err)
	secondSHA := resolveHead(t, ctx, repo)
	require.NotEqual(t, firstSHA, secondSHA, "HEAD must have advanced")

	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	// Pass the OLD SHA — Provision must use it, not current HEAD.
	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir, firstSHA, testRuntimeIdentity)
	require.NoError(t, err)

	// workspace.json must record the caller-provided SHA.
	data, err := os.ReadFile(filepath.Join(evidenceDir, "workspace.json"))
	require.NoError(t, err)

	var m Metadata
	require.NoError(t, json.Unmarshal(data, &m))
	require.Equal(t, firstSHA, m.BaseSHA)

	// The worktree must be checked out at the caller-provided SHA.
	out, err := exec.CommandContext(ctx, "git", "-C", wsPath, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	require.Equal(t, firstSHA, strings.TrimSpace(string(out)))
}

func TestCleanup_Integration_RemovesWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	require.NoError(t, Cleanup(ctx, homeDir, repo.Dir(), wsPath))

	_, err = os.Stat(wsPath)
	require.True(t, os.IsNotExist(err))
}

func TestCleanup_Integration_Idempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	require.NoError(t, Cleanup(ctx, homeDir, repo.Dir(), wsPath))
	require.NoError(t, Cleanup(ctx, homeDir, repo.Dir(), wsPath))
}

func TestCleanup_Integration_GitWorktreeListClean(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAX", evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	require.NoError(t, Cleanup(ctx, homeDir, repo.Dir(), wsPath))

	// git worktree list must show only the main worktree after cleanup.
	out, err := exec.CommandContext(ctx, "git", "-C", repo.Dir(), "worktree", "list", "--porcelain").CombinedOutput()
	require.NoError(t, err, "git worktree list: %s", out)

	worktreeCount := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			worktreeCount++
		}
	}
	require.Equal(t, 1, worktreeCount, "only the main worktree should remain after cleanup, got: %s", out)
}

func TestCleanup_Integration_RemovesWorktreeRef_WhenRepairFails(t *testing.T) {
	// No t.Parallel() — t.Setenv is not compatible with parallel tests.

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAR", evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	// Simulate Docker unavailability by prepending a directory with a fake
	// docker binary that always fails, so repairWorkspaceOwnership errors out
	// while git remains available.
	fakeDockerDir := t.TempDir()
	fakePath := filepath.Join(fakeDockerDir, "docker")
	require.NoError(t, os.WriteFile(fakePath, []byte("#!/bin/sh\nexit 1\n"), 0o755))
	t.Setenv("PATH", fakeDockerDir+":"+os.Getenv("PATH"))

	err = Cleanup(ctx, homeDir, repo.Dir(), wsPath)
	require.NoError(t, err, "cleanup must succeed even when Docker repair fails")

	// Verify the workspace directory is gone.
	_, statErr := os.Stat(wsPath)
	require.True(t, os.IsNotExist(statErr), "workspace directory must be removed")

	// Verify git worktree ref is cleaned up.
	out, err := exec.CommandContext(ctx, "git", "-C", repo.Dir(), "worktree", "list", "--porcelain").CombinedOutput()
	require.NoError(t, err, "git worktree list: %s", out)

	worktreeCount := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			worktreeCount++
		}
	}
	require.Equal(t, 1, worktreeCount, "only main worktree should remain, got: %s", out)
}

func TestProvision_Integration_WorktreeMode_NoOtherAccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5F01", evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	// Every file and directory inside the worktree must strip world bits.
	err = filepath.WalkDir(wsPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		perm := info.Mode().Perm()
		require.Equal(t, os.FileMode(0), perm&0o007,
			"%s: world bits must be clear, got %o", path, perm)
		return nil
	})
	require.NoError(t, err)

	// Parent chain must be owner-only (0o700) so second users cannot enumerate
	// run IDs.
	for _, p := range []string{
		filepath.Join(homeDir, ".tessariq"),
		filepath.Join(homeDir, ".tessariq", "worktrees"),
		filepath.Dir(wsPath),
	} {
		info, statErr := os.Stat(p)
		require.NoError(t, statErr, p)
		require.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
			"%s: parent chain must be 0700", p)
	}
}

func TestProvision_Integration_ContainerUserCanWrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	// Parent chain is 0o700 owned by host user; ensure the sibling
	// container process can traverse by mounting the worktree directly.
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5F02", evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	// Write as the container's resolved runtime uid/gid. Exact-principal ACLs
	// must grant this user write access without reopening the path to a host gid.
	userFlag := fmt.Sprintf("%d:%d", testRuntimeIdentity.UID, testRuntimeIdentity.GID)
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--user", userFlag,
		"-v", wsPath+":/work",
		"alpine:latest",
		"sh", "-c", "touch /work/probe && echo hi > /work/probe && cat /work/probe",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "container user must retain write access: %s", out)
	require.Contains(t, string(out), "hi")
}

func TestProvision_Integration_ThirdPartyUserCannotRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5F03", evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	// A third UID (not the host UID and not the runtime uid) must be
	// denied read access to worktree contents. `.git` in a worktree is a
	// file (gitdir link), which we use as the probe target.
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--user", "4242:4242",
		"-v", wsPath+":/work",
		"alpine:latest",
		"sh", "-c", "cat /work/.git",
	)
	out, err := cmd.CombinedOutput()
	require.Error(t, err, "third-party user must be denied: %s", out)
	require.Contains(t, strings.ToLower(string(out)), "permission denied",
		"expected permission-denied error, got: %s", out)
}

func TestProvision_Integration_CustomRuntimeUIDCanWrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	identity := container.RuntimeIdentity{UID: 1234, GID: 1234}

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5F04", evidenceDir, headSHA, identity)
	require.NoError(t, err)

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--user", "1234:1234",
		"-v", wsPath+":/work",
		"alpine:latest",
		"sh", "-c", "touch /work/probe && echo hi > /work/probe && cat /work/probe",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "custom runtime uid must retain write access: %s", out)
	require.Contains(t, string(out), "hi")
}

func TestProvision_Integration_HardeningFailureCleansWorktree(t *testing.T) {
	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	wsPath := WorkspacePath(homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5F05")

	old := hardenWorktreePath
	hardenWorktreePath = func(_ context.Context, _ string, _ container.RuntimeIdentity) error {
		return errors.New("boom")
	}
	t.Cleanup(func() { hardenWorktreePath = old })

	_, err = Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5F05", evidenceDir, headSHA, testRuntimeIdentity)
	require.Error(t, err)

	_, statErr := os.Stat(wsPath)
	require.True(t, os.IsNotExist(statErr), "worktree must be cleaned on hardening failure")
}

func TestCleanup_Integration_RemovesRestrictiveContainerOwnedFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo, err := containers.StartGitRepo(ctx, t)
	require.NoError(t, err)

	headSHA := resolveHead(t, ctx, repo)
	homeDir := t.TempDir()
	evidenceDir := filepath.Join(t.TempDir(), "evidence")

	wsPath, err := Provision(ctx, homeDir, repo.Dir(), "01ARZ3NDEKTSV4RRFFQ69G5FAW", evidenceDir, headSHA, testRuntimeIdentity)
	require.NoError(t, err)

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-v", wsPath+":/work", "alpine:latest",
		"sh", "-c", "mkdir -p /work/private && touch /work/private/file && chmod 700 /work/private")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "create restrictive files: %s", out)

	require.NoError(t, Cleanup(ctx, homeDir, repo.Dir(), wsPath))

	_, err = os.Stat(wsPath)
	require.True(t, os.IsNotExist(err))
}
