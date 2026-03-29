package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTaskPathLogic_NotMarkdown(t *testing.T) {
	t.Parallel()

	err := ValidateTaskPathLogic("/repo", "tasks/script.sh")
	require.EqualError(t, err, "task path must be a Markdown file: tasks/script.sh")
}

func TestValidateTaskPathLogic_OutsideRepo(t *testing.T) {
	t.Parallel()

	err := ValidateTaskPathLogic("/repo", "/etc/passwd.md")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be relative to the repository")
}

func TestValidateTaskPathLogic_RelativeEscape(t *testing.T) {
	t.Parallel()

	err := ValidateTaskPathLogic("/repo", "../../etc/evil.md")
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside the repository")
}

func TestValidateTaskPathLogic_ValidRelativePath(t *testing.T) {
	t.Parallel()

	err := ValidateTaskPathLogic("/repo", "specs/example.md")
	require.NoError(t, err)
}

func TestValidateTaskPathLogic_SubdirectoryMarkdown(t *testing.T) {
	t.Parallel()

	err := ValidateTaskPathLogic("/repo", "deep/nested/path/task.md")
	require.NoError(t, err)
}

func TestValidateTaskPath_MissingFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	err := ValidateTaskPath(root, "nonexistent.md")
	require.Error(t, err)
	require.Contains(t, err.Error(), "task path does not exist")
}

func TestValidateTaskPath_NotRegularFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dirPath := filepath.Join(root, "subdir.md")
	require.NoError(t, os.Mkdir(dirPath, 0o755))

	err := ValidateTaskPath(root, "subdir.md")
	require.Error(t, err)
	require.Contains(t, err.Error(), "task path is not a regular file")
}

func TestValidateTaskPath_ValidFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "specs"), 0o755))
	taskFile := filepath.Join(root, "specs", "example.md")
	require.NoError(t, os.WriteFile(taskFile, []byte("# Task"), 0o644))

	err := ValidateTaskPath(root, filepath.Join("specs", "example.md"))
	require.NoError(t, err)
}
