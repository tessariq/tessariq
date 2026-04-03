package git

import (
	"context"
	"fmt"
	"os/exec"
)

// intentToAdd marks all untracked files in the worktree so they appear in
// subsequent diff commands. This is a no-op when there are no untracked files.
func intentToAdd(ctx context.Context, repoRoot string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "add", "-N", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add -N: %s: %w", out, err)
	}
	return nil
}

// Diff returns the unified diff between a base commit and the current
// worktree state, including untracked files. Returns empty output (not
// an error) when there are no changes.
func Diff(ctx context.Context, repoRoot, baseSHA string) ([]byte, error) {
	if err := intentToAdd(ctx, repoRoot); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff", "--binary", baseSHA, "--", ".")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return out, nil
}

// DiffStat returns the diffstat summary between a base commit and the
// current worktree state, including untracked files. Returns empty output
// when there are no changes.
func DiffStat(ctx context.Context, repoRoot, baseSHA string) ([]byte, error) {
	if err := intentToAdd(ctx, repoRoot); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff", "--stat", baseSHA, "--", ".")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diffstat: %w", err)
	}
	return out, nil
}
