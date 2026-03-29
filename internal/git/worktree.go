package git

import (
	"context"
	"fmt"
	"os/exec"
)

// AddWorktree creates a detached worktree at the given path checked out at commitish.
func AddWorktree(ctx context.Context, repoRoot, path, commitish string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "add", "--detach", path, commitish)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add worktree at %s: %s: %w", path, out, err)
	}
	return nil
}

// RemoveWorktree removes a worktree from the repository. The --force flag
// allows removal even when the worktree has uncommitted changes.
func RemoveWorktree(ctx context.Context, repoRoot, path string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "remove", "--force", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove worktree at %s: %s: %w", path, out, err)
	}
	return nil
}
