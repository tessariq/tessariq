package initialize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const gitignoreEntry = ".tessariq/"

// evidenceDirMode is the permission mode for evidence parent directories.
// Owner-only access per the evidence permission contract.
const evidenceDirMode = 0o700

// Run creates the tessariq runtime state directory and ensures .tessariq/ is
// in .gitignore. It is idempotent and tightens permissions on re-run.
func Run(repoRoot string) error {
	tessariqDir := filepath.Join(repoRoot, ".tessariq")
	runsDir := filepath.Join(tessariqDir, "runs")
	if err := os.MkdirAll(runsDir, evidenceDirMode); err != nil {
		return fmt.Errorf("create .tessariq/runs: %w", err)
	}
	// Explicit chmod ensures idempotent tightening: MkdirAll only sets mode
	// on newly created directories, so re-runs on existing dirs need this.
	for _, d := range []string{tessariqDir, runsDir} {
		if err := os.Chmod(d, evidenceDirMode); err != nil {
			return fmt.Errorf("set permissions on %s: %w", d, err)
		}
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
