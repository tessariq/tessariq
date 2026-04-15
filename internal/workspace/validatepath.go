package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrWorkspacePathOutsideTree is returned when a workspace path is not
// contained within the canonical <homeDir>/.tessariq/worktrees/ tree, or
// does not resolve to the canonical per-run path derived from trusted
// inputs.
var ErrWorkspacePathOutsideTree = errors.New("workspace path is outside the canonical worktrees tree")

// ValidateWorkspacePath asserts that untrustedPath (typically read out of
// workspace.json) refers to the same real filesystem target as the
// canonical per-run workspace path derived from trusted inputs via
// WorkspacePath. Symlinks on both the homeDir and the untrusted path are
// resolved before the containment check so a symlink planted under
// ~/.tessariq/worktrees/ cannot be used to escape the tree.
//
// An empty untrustedPath returns ("", nil) so callers can keep their
// "no workspace.json => no cleanup" semantics.
//
// On success it returns the lexical canonical path (stable across
// platforms; macOS in particular would otherwise produce a
// /private/var/... form from EvalSymlinks that differs from the
// /var/... lexical canonical). The containment check is still performed
// against the symlink-resolved form internally.
func ValidateWorkspacePath(homeDir, repoRoot, runID, untrustedPath string) (string, error) {
	if untrustedPath == "" {
		return "", nil
	}
	if !filepath.IsAbs(untrustedPath) {
		return "", fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, untrustedPath)
	}

	canonical := filepath.Clean(WorkspacePath(homeDir, repoRoot, runID))
	if filepath.Clean(untrustedPath) != canonical {
		return "", fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, untrustedPath)
	}

	realHome, err := filepath.EvalSymlinks(filepath.Clean(homeDir))
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, untrustedPath)
	}
	realWorktreesPrefix := filepath.Join(realHome, ".tessariq", "worktrees") + string(filepath.Separator)

	realPath, err := filepath.EvalSymlinks(untrustedPath)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, untrustedPath)
	}
	if !strings.HasPrefix(realPath+string(filepath.Separator), realWorktreesPrefix) {
		return "", fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, untrustedPath)
	}

	realCanonical, err := filepath.EvalSymlinks(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, untrustedPath)
	}
	if realPath != realCanonical {
		return "", fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, untrustedPath)
	}

	return canonical, nil
}

// assertInsideWorktrees is the defensive safety net used by Cleanup. It
// refuses any path that is not an absolute path contained under
// <homeDir>/.tessariq/worktrees/ after symlink resolution. Unlike
// ValidateWorkspacePath, it does not require a canonical per-run match —
// it just enforces the broader containment envelope so a caller that
// forgot to validate cannot weaponize Cleanup into an arbitrary-path
// primitive.
//
// Non-existent paths are accepted lexically so the idempotent "already
// cleaned" branch in Cleanup still works.
func assertInsideWorktrees(homeDir, workspacePath string) error {
	if !filepath.IsAbs(workspacePath) {
		return fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, workspacePath)
	}

	cleanedPath := filepath.Clean(workspacePath)

	realHome, err := filepath.EvalSymlinks(filepath.Clean(homeDir))
	if err != nil {
		return fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, workspacePath)
	}
	realWorktreesPrefix := filepath.Join(realHome, ".tessariq", "worktrees") + string(filepath.Separator)

	// If the path already exists, enforce real-path containment.
	if _, statErr := os.Lstat(cleanedPath); statErr == nil {
		realPath, err := filepath.EvalSymlinks(cleanedPath)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, workspacePath)
		}
		if !strings.HasPrefix(realPath+string(filepath.Separator), realWorktreesPrefix) {
			return fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, workspacePath)
		}
		return nil
	}

	// Path does not exist — fall back to a lexical containment check so
	// the no-op idempotent branch still works without resolving a
	// non-existent target.
	lexicalWorktreesPrefix := filepath.Join(filepath.Clean(homeDir), ".tessariq", "worktrees") + string(filepath.Separator)
	if !strings.HasPrefix(cleanedPath+string(filepath.Separator), lexicalWorktreesPrefix) &&
		!strings.HasPrefix(cleanedPath+string(filepath.Separator), realWorktreesPrefix) {
		return fmt.Errorf("%w: %s", ErrWorkspacePathOutsideTree, workspacePath)
	}
	return nil
}
