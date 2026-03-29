package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tessariq/tessariq/internal/git"
)

// WorkspacePath computes the deterministic path for a worktree workspace:
// <homeDir>/.tessariq/worktrees/<repo_id>/<run_id>
func WorkspacePath(homeDir, repoRoot, runID string) string {
	return filepath.Join(homeDir, ".tessariq", "worktrees", RepoID(repoRoot), runID)
}

// Provision creates a detached worktree at the computed workspace path and
// writes workspace.json into the evidence directory. It returns the workspace
// path and the base SHA used.
func Provision(ctx context.Context, homeDir, repoRoot, runID, evidenceDir string) (string, string, error) {
	baseSHA, err := git.HeadSHA(ctx, repoRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve base sha: %w", err)
	}

	wsPath := WorkspacePath(homeDir, repoRoot, runID)

	if err := os.MkdirAll(filepath.Dir(wsPath), 0o755); err != nil {
		return "", "", fmt.Errorf("create worktree parent: %w", err)
	}

	if err := git.AddWorktree(ctx, repoRoot, wsPath, baseSHA); err != nil {
		return "", "", fmt.Errorf("provision worktree: %w", err)
	}

	m := BuildMetadata(baseSHA, wsPath)
	if err := WriteMetadata(evidenceDir, m); err != nil {
		return "", "", fmt.Errorf("write workspace metadata: %w", err)
	}

	return wsPath, baseSHA, nil
}

// Cleanup removes the worktree and its directory. It is safe to call multiple
// times — a missing worktree or directory is not an error.
func Cleanup(ctx context.Context, repoRoot, workspacePath string) error {
	if err := git.RemoveWorktree(ctx, repoRoot, workspacePath); err != nil {
		if _, statErr := os.Stat(workspacePath); os.IsNotExist(statErr) {
			return nil
		}
		return fmt.Errorf("cleanup worktree: %w", err)
	}
	_ = os.RemoveAll(workspacePath)
	return nil
}
