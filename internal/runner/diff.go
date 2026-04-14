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
// Skips both files when there are no changes. Commits are all-or-nothing:
// if any step fails, any already-written artifact or temporary file is
// removed so the caller never sees a partial evidence set.
func WriteDiffArtifacts(ctx context.Context, evidenceDir, worktreePath, baseSHA string) (retErr error) {
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

	patchPath := filepath.Join(evidenceDir, "diff.patch")
	statPath := filepath.Join(evidenceDir, "diffstat.txt")
	patchTmp := patchPath + ".tmp"
	statTmp := statPath + ".tmp"

	// patchCommitted tracks whether diff.patch was renamed into place so we
	// can roll it back if the subsequent diffstat.txt commit fails.
	patchCommitted := false

	// Clean up any residual temp or committed files on failure so the
	// evidence directory is never left with partial diff artifacts.
	defer func() {
		if retErr == nil {
			return
		}
		_ = os.Remove(patchTmp)
		_ = os.Remove(statTmp)
		if patchCommitted {
			_ = os.Remove(patchPath)
		}
	}()

	if err := os.WriteFile(patchTmp, patch, 0o600); err != nil {
		return fmt.Errorf("write diff.patch temp file: %w", err)
	}

	if err := os.WriteFile(statTmp, stat, 0o600); err != nil {
		return fmt.Errorf("write diffstat.txt temp file: %w", err)
	}

	if err := os.Rename(patchTmp, patchPath); err != nil {
		return fmt.Errorf("commit diff.patch: %w", err)
	}
	patchCommitted = true

	if err := os.Rename(statTmp, statPath); err != nil {
		return fmt.Errorf("commit diffstat.txt: %w", err)
	}

	return nil
}
