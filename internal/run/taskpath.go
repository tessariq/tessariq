package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ValidateTaskPath(repoRoot, taskPath string) error {
	if err := ValidateTaskPathLogic(repoRoot, taskPath); err != nil {
		return err
	}

	absPath := filepath.Join(repoRoot, taskPath)
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("task path does not exist: %s", taskPath)
		}
		return fmt.Errorf("stat task path %s: %w", taskPath, err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("task path is not a regular file: %s", taskPath)
	}

	return nil
}

func ValidateTaskPathLogic(repoRoot, taskPath string) error {
	if filepath.IsAbs(taskPath) {
		return fmt.Errorf("task path must be relative to the repository: %s", taskPath)
	}

	if !strings.HasSuffix(strings.ToLower(taskPath), ".md") {
		return fmt.Errorf("task path must be a Markdown file: %s", taskPath)
	}

	absPath := filepath.Clean(filepath.Join(repoRoot, taskPath))
	cleanRoot := filepath.Clean(repoRoot)

	if !strings.HasPrefix(absPath, cleanRoot+string(filepath.Separator)) && absPath != cleanRoot {
		return fmt.Errorf("task path is outside the repository: %s", taskPath)
	}

	return nil
}
