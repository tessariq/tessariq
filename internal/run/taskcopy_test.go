package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyTaskFile_CopiesExactly(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "specs"), 0o755))
	taskContent := []byte("# Task\n\nSome body content")
	taskFile := filepath.Join(repoRoot, "specs", "task.md")
	require.NoError(t, os.WriteFile(taskFile, taskContent, 0o644))

	evidenceDir := t.TempDir()

	require.NoError(t, CopyTaskFile(repoRoot, "specs/task.md", evidenceDir, taskContent))

	dest := filepath.Join(evidenceDir, "task.md")
	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, taskContent, data)
}

func TestCopyTaskFile_PreservesPermissions(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	taskContent := []byte("# Task")
	taskFile := filepath.Join(repoRoot, "task.md")
	require.NoError(t, os.WriteFile(taskFile, taskContent, 0o600))

	evidenceDir := t.TempDir()

	require.NoError(t, CopyTaskFile(repoRoot, "task.md", evidenceDir, taskContent))

	dest := filepath.Join(evidenceDir, "task.md")
	info, err := os.Stat(dest)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestCopyTaskFile_NestedPath(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "deep", "nested"), 0o755))
	taskContent := []byte("# Deep Task")
	taskFile := filepath.Join(repoRoot, "deep", "nested", "task.md")
	require.NoError(t, os.WriteFile(taskFile, taskContent, 0o644))

	evidenceDir := t.TempDir()

	require.NoError(t, CopyTaskFile(repoRoot, "deep/nested/task.md", evidenceDir, taskContent))

	dest := filepath.Join(evidenceDir, "task.md")
	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, taskContent, data)
}

func TestCopyTaskFile_SourceNotFound(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	evidenceDir := t.TempDir()

	err := CopyTaskFile(repoRoot, "nonexistent.md", evidenceDir, []byte("x"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "read task file")
}

func TestCopyTaskFile_TargetDirNotFound(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	taskFile := filepath.Join(repoRoot, "task.md")
	require.NoError(t, os.WriteFile(taskFile, []byte("# T"), 0o644))

	evidenceDir := filepath.Join(t.TempDir(), "sub", "dir")

	err := CopyTaskFile(repoRoot, "task.md", evidenceDir, []byte("# T"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "write task file")
}

func TestCopyTaskFile_ContentMismatchIgnored(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	taskContent := []byte("# Original")
	taskFile := filepath.Join(repoRoot, "task.md")
	require.NoError(t, os.WriteFile(taskFile, taskContent, 0o644))

	evidenceDir := t.TempDir()
	differentContent := []byte("# Replaced")

	require.NoError(t, CopyTaskFile(repoRoot, "task.md", evidenceDir, differentContent))

	dest := filepath.Join(evidenceDir, "task.md")
	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, differentContent, data)
}
