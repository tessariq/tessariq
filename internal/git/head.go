package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// HeadSHA returns the full SHA of HEAD in the given repository.
func HeadSHA(ctx context.Context, repoRoot string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
