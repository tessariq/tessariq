package run

import (
	"fmt"
	"os"
	"path/filepath"
)

func CopyTaskFile(repoRoot, taskPath, evidenceDir string, content []byte) error {
	src := filepath.Join(repoRoot, taskPath)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("read task file: %w", err)
	}

	dest := filepath.Join(evidenceDir, "task.md")
	if err := os.WriteFile(dest, content, 0o600); err != nil {
		return fmt.Errorf("write task file: %w", err)
	}

	return nil
}
