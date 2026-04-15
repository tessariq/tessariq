package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tessariq/tessariq/internal/git"
)

// repairImage is the container image used to fix workspace file ownership.
// Pinned by digest to prevent supply-chain attacks — update the digest when
// upgrading Alpine.
const repairImage = "alpine@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659"

// WorkspacePath computes the deterministic path for a worktree workspace:
// <homeDir>/.tessariq/worktrees/<repo_id>/<run_id>
func WorkspacePath(homeDir, repoRoot, runID string) string {
	return filepath.Join(homeDir, ".tessariq", "worktrees", RepoID(repoRoot), runID)
}

// Provision creates a detached worktree at the computed workspace path and
// writes workspace.json into the evidence directory. The caller must supply the
// base SHA so that all evidence artifacts use a single, consistent value.
func Provision(ctx context.Context, homeDir, repoRoot, runID, evidenceDir, baseSHA string) (string, error) {
	wsPath := WorkspacePath(homeDir, repoRoot, runID)

	if err := os.MkdirAll(filepath.Dir(wsPath), 0o755); err != nil {
		return "", fmt.Errorf("create worktree parent: %w", err)
	}

	if err := git.AddWorktree(ctx, repoRoot, wsPath, baseSHA); err != nil {
		return "", fmt.Errorf("provision worktree: %w", err)
	}

	m := BuildMetadata(baseSHA, wsPath)
	if err := WriteMetadata(evidenceDir, m); err != nil {
		return "", fmt.Errorf("write workspace metadata: %w", err)
	}

	return wsPath, nil
}

// Cleanup removes the worktree and its directory. It is safe to call multiple
// times — a missing worktree or directory is not an error.
//
// Before removal, it reclaims permissions with chmod so the host user can
// traverse and delete files that may have been created by the container's
// non-root user (different UID than the host user). If Docker-based repair
// fails, a host-side chmod fallback is attempted. Neither failure prevents
// the subsequent git worktree removal and filesystem deletion.
//
// workspacePath is treated as untrusted defensive input and must resolve
// inside <homeDir>/.tessariq/worktrees/ after symlink resolution. Paths
// outside the canonical tree are rejected with ErrWorkspacePathOutsideTree
// before any chown, chmod, or removal step runs.
func Cleanup(ctx context.Context, homeDir, repoRoot, workspacePath string) error {
	if err := assertInsideWorktrees(homeDir, workspacePath); err != nil {
		return err
	}
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return nil
	}

	// Best-effort ownership repair: Docker container first, host chmod fallback.
	var repairErr error
	if err := repairWorkspaceOwnership(ctx, workspacePath); err != nil {
		repairErr = err
		_ = hostChmod(workspacePath)
	}

	// Remove the git worktree ref regardless of repair outcome.
	_ = git.RemoveWorktree(ctx, repoRoot, workspacePath)

	// Remove the filesystem directory.
	_ = os.RemoveAll(workspacePath)

	// If the directory still exists, cleanup failed — report why.
	if _, err := os.Stat(workspacePath); err == nil {
		if repairErr != nil {
			return fmt.Errorf("cleanup worktree %s: ownership repair failed and directory still exists: %w", workspacePath, repairErr)
		}
		return fmt.Errorf("cleanup worktree %s: directory still exists after removal attempt", workspacePath)
	}

	return nil
}

// hostChmod attempts a host-side permission repair when the Docker-based
// repair container is unavailable.
func hostChmod(workspacePath string) error {
	cmd := exec.Command("chmod", "-R", "u+rwX", workspacePath)
	return cmd.Run()
}

// buildRepairArgs assembles the docker run arguments for workspace ownership repair.
func buildRepairArgs(workspacePath string) []string {
	uid := os.Getuid()
	gid := os.Getgid()
	fixCmd := fmt.Sprintf("chown -R %d:%d /work && chmod -R u+rwX /work", uid, gid)
	return []string{
		"run", "--rm", "--user", "root",
		"-v", workspacePath + ":/work",
		repairImage, "sh", "-c", fixCmd,
	}
}

func repairWorkspaceOwnership(ctx context.Context, workspacePath string) error {
	args := buildRepairArgs(workspacePath)
	cmd := exec.CommandContext(ctx, "docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("repair workspace ownership for %s: %s: %w", workspacePath, strings.TrimSpace(string(out)), err)
	}
	return nil
}
