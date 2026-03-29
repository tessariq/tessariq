package initialize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const gitignoreEntry = ".tessariq/"

// Run creates the tessariq runtime state directory and ensures .tessariq/ is
// in .gitignore. It is idempotent.
func Run(repoRoot string) error {
	runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return fmt.Errorf("create .tessariq/runs: %w", err)
	}

	if err := ensureGitignoreEntry(repoRoot); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}

	return nil
}

func ensureGitignoreEntry(repoRoot string) error {
	path := filepath.Join(repoRoot, ".gitignore")

	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	text := string(content)
	if containsLine(text, gitignoreEntry) {
		return nil
	}

	var buf strings.Builder
	buf.WriteString(text)
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		buf.WriteByte('\n')
	}
	buf.WriteString(gitignoreEntry)
	buf.WriteByte('\n')

	return os.WriteFile(path, []byte(buf.String()), 0o644)
}

func containsLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}
