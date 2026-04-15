package promote

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tessariq/tessariq/internal/lifecycle"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
)

var (
	ErrNoCodeChanges            = errors.New("there were no code changes to promote")
	ErrRunNotFinished           = errors.New("run is not finished")
	ErrBranchExists             = errors.New("branch already exists")
	ErrManifestIdentityMismatch = errors.New("manifest identity does not match resolved run")
)

type Options struct {
	RunRef     string
	Branch     string
	Message    string
	NoTrailers bool
}

type Result struct {
	RunID  string
	Branch string
	Commit string
}

func Run(ctx context.Context, repoRoot string, opts Options) (Result, error) {
	runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
	entry, err := run.ResolveRunRef(runsDir, opts.RunRef)
	if err != nil {
		return Result{}, err
	}

	evidenceDir, err := run.ValidateEvidencePath(repoRoot, entry.EvidencePath)
	if err != nil {
		return Result{}, fmt.Errorf("run evidence is invalid or outside the repository: %w", err)
	}

	if err := runner.CheckEvidenceCompleteness(evidenceDir); err != nil {
		return Result{}, fmt.Errorf("required evidence is missing or incomplete; the run cannot be promoted until evidence is intact: %w", err)
	}

	manifest, err := run.ReadManifest(evidenceDir)
	if err != nil {
		return Result{}, err
	}

	if err := validateManifestIdentity(entry, manifest, evidenceDir); err != nil {
		return Result{}, err
	}

	status, err := runner.ReadStatus(evidenceDir)
	if err != nil {
		return Result{}, err
	}
	if !status.State.IsTerminal() {
		reconciled, err := lifecycle.ReconcileRun(ctx, repoRoot, entry)
		if err != nil {
			return Result{}, err
		}
		entry = reconciled.Entry
		status = reconciled.Status
		if !status.State.IsTerminal() {
			return Result{}, fmt.Errorf("%w: run %s is in state %s", ErrRunNotFinished, manifest.RunID, status.State)
		}
	}

	patchPath := filepath.Join(evidenceDir, "diff.patch")
	hasPatch, err := hasNonEmptyFile(patchPath, "diff.patch")
	if err != nil {
		return Result{}, err
	}
	if !hasPatch {
		return Result{}, fmt.Errorf("run %s: %w", manifest.RunID, ErrNoCodeChanges)
	}

	diffstatPath := filepath.Join(evidenceDir, "diffstat.txt")
	hasStat, err := hasNonEmptyFile(diffstatPath, "diffstat.txt")
	if err != nil {
		return Result{}, err
	}
	if !hasStat {
		return Result{}, fmt.Errorf("required evidence is missing or incomplete; the run cannot be promoted until evidence is intact: incomplete evidence: diffstat.txt")
	}

	branch := resolveBranchName(manifest.RunID, opts.Branch)
	if err := validateBranchName(ctx, repoRoot, branch); err != nil {
		return Result{}, err
	}
	if err := ensureBranchDoesNotExist(ctx, repoRoot, branch); err != nil {
		return Result{}, err
	}

	commitMessage := resolveCommitMessage(manifest, opts.Message)
	messageBody, err := buildCommitMessage(commitMessage, manifest, !opts.NoTrailers)
	if err != nil {
		return Result{}, err
	}

	tmpDir, err := os.MkdirTemp("", "tessariq-promote-*")
	if err != nil {
		return Result{}, fmt.Errorf("create temporary promote directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	worktreePath := filepath.Join(tmpDir, "worktree")
	if err := gitRun(ctx, repoRoot, "worktree", "add", "--detach", worktreePath, manifest.BaseSHA); err != nil {
		return Result{}, fmt.Errorf("create temporary worktree: %w", err)
	}
	defer func() {
		_ = gitRun(context.Background(), repoRoot, "worktree", "remove", "--force", worktreePath)
	}()

	if err := gitRun(ctx, worktreePath, "apply", patchPath); err != nil {
		return Result{}, fmt.Errorf("apply promote diff: %w", err)
	}
	if err := gitRun(ctx, worktreePath, "add", "-A"); err != nil {
		return Result{}, fmt.Errorf("stage promoted changes: %w", err)
	}

	hasChanges, err := hasStagedChanges(ctx, worktreePath)
	if err != nil {
		return Result{}, err
	}
	if !hasChanges {
		return Result{}, fmt.Errorf("run %s: %w", manifest.RunID, ErrNoCodeChanges)
	}

	messagePath := filepath.Join(tmpDir, "commit-message.txt")
	if err := os.WriteFile(messagePath, []byte(messageBody), 0o600); err != nil {
		return Result{}, fmt.Errorf("write commit message: %w", err)
	}
	if err := gitRun(ctx, worktreePath, "commit", "-F", messagePath); err != nil {
		return Result{}, fmt.Errorf("create promote commit: %w", err)
	}

	commitSHA, err := gitOutput(ctx, worktreePath, "rev-parse", "HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("resolve promoted commit: %w", err)
	}
	if err := gitRun(ctx, repoRoot, "branch", branch, commitSHA); err != nil {
		return Result{}, fmt.Errorf("create promote branch: %w", err)
	}

	return Result{RunID: manifest.RunID, Branch: branch, Commit: commitSHA}, nil
}

func validateManifestIdentity(entry run.IndexEntry, manifest run.Manifest, evidenceDir string) error {
	if manifest.RunID != entry.RunID {
		return fmt.Errorf("%w: manifest run_id %q does not match resolved run %q; evidence may be inconsistent or tampered",
			ErrManifestIdentityMismatch, manifest.RunID, entry.RunID)
	}
	dirName := filepath.Base(evidenceDir)
	if manifest.RunID != dirName {
		return fmt.Errorf("%w: manifest run_id %q does not match evidence directory %q; evidence may be inconsistent or tampered",
			ErrManifestIdentityMismatch, manifest.RunID, dirName)
	}
	return nil
}

func defaultBranchName(runID string) string {
	return "tessariq/" + runID
}

func resolveBranchName(runID, branch string) string {
	if strings.TrimSpace(branch) != "" {
		return branch
	}
	return defaultBranchName(runID)
}

func defaultCommitMessage(taskTitle, runID string) string {
	if strings.TrimSpace(taskTitle) != "" {
		return taskTitle
	}
	return fmt.Sprintf("tessariq: apply run %s", runID)
}

func resolveCommitMessage(manifest run.Manifest, message string) string {
	if message != "" {
		return message
	}
	return defaultCommitMessage(manifest.TaskTitle, manifest.RunID)
}

func buildCommitMessage(message string, manifest run.Manifest, includeTrailers bool) (string, error) {
	if run.ContainsControlChar(manifest.TaskPath) {
		return "", fmt.Errorf("manifest task_path must not contain control characters: refusing to build commit trailer")
	}
	if run.ContainsControlChar(manifest.TaskTitle) {
		return "", fmt.Errorf("manifest task_title must not contain control characters: refusing to build commit trailer")
	}

	if !includeTrailers {
		return message + "\n", nil
	}

	return fmt.Sprintf("%s\n\nTessariq-Run: %s\nTessariq-Base: %s\nTessariq-Task: %s\n",
		message,
		manifest.RunID,
		manifest.BaseSHA,
		manifest.TaskPath,
	), nil
}

// hasNonEmptyFile reports whether the file at path exists and has non-zero size.
// Returns (false, nil) when the file is missing or empty.
func hasNonEmptyFile(path, name string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", name, err)
	}

	return info.Size() > 0, nil
}

func validateBranchName(ctx context.Context, repoRoot, branch string) error {
	if strings.TrimSpace(branch) == "" {
		return fmt.Errorf("invalid branch name %q", branch)
	}
	if err := gitRun(ctx, repoRoot, "check-ref-format", "--branch", branch); err != nil {
		return fmt.Errorf("invalid branch name %q: %w", branch, err)
	}
	return nil
}

func ensureBranchDoesNotExist(ctx context.Context, repoRoot, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	err := cmd.Run()
	if err == nil {
		return fmt.Errorf("%w: %s", ErrBranchExists, branch)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check existing branch %q: %w", branch, err)
	}
	return nil
}

func hasStagedChanges(ctx context.Context, repoRoot string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err == nil {
		return false, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("check staged diff: %w", err)
}

func gitRun(ctx context.Context, repoRoot string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoRoot}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return err
		}
		return fmt.Errorf("%s: %w", trimmed, err)
	}
	return nil
}

func gitOutput(ctx context.Context, repoRoot string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoRoot}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return "", err
		}
		return "", fmt.Errorf("%s: %w", trimmed, err)
	}
	return strings.TrimSpace(string(out)), nil
}
