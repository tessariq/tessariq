package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrDirtyRepo = errors.New("repository is dirty; commit, stash, or clean the repository first")

func IsClean(ctx context.Context, repoRoot string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	cmd = exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "HEAD")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: repository has no commits", ErrDirtyRepo)
	}

	cmd = exec.CommandContext(ctx, "git", "-C", repoRoot, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}

	return parsePorcelain(string(out))
}

func parsePorcelain(output string) error {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var entries []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			entries = append(entries, line)
		}
	}

	if len(entries) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s", ErrDirtyRepo, strings.Join(entries, ", "))
}
