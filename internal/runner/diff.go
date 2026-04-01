package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tessariq/tessariq/internal/git"
)

// WriteDiffArtifacts generates diff.patch and diffstat.txt in the evidence
// directory when changes exist in the worktree relative to baseSHA.
// Skips both files when there are no changes.
func WriteDiffArtifacts(ctx context.Context, evidenceDir, worktreePath, baseSHA string) error {
	patch, err := git.Diff(ctx, worktreePath, baseSHA)
	if err != nil {
		return fmt.Errorf("generate diff: %w", err)
	}

	if len(patch) == 0 {
		return nil
	}

	stat, err := git.DiffStat(ctx, worktreePath, baseSHA)
	if err != nil {
		return fmt.Errorf("generate diffstat: %w", err)
	}

	if err := os.WriteFile(filepath.Join(evidenceDir, "diff.patch"), patch, 0o600); err != nil {
		return fmt.Errorf("write diff.patch: %w", err)
	}

	if err := os.WriteFile(filepath.Join(evidenceDir, "diffstat.txt"), stat, 0o600); err != nil {
		return fmt.Errorf("write diffstat.txt: %w", err)
	}

	return nil
}
