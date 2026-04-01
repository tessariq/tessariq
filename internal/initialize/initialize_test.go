package initialize

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainsLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		line    string
		want    bool
	}{
		{
			name:    "exact match",
			content: ".tessariq/\n",
			line:    ".tessariq/",
			want:    true,
		},
		{
			name:    "match among other lines",
			content: "dist/\n.tessariq/\n*.out\n",
			line:    ".tessariq/",
			want:    true,
		},
		{
			name:    "no match",
			content: "dist/\n*.out\n",
			line:    ".tessariq/",
			want:    false,
		},
		{
			name:    "empty content",
			content: "",
			line:    ".tessariq/",
			want:    false,
		},
		{
			name:    "line with surrounding whitespace",
			content: "  .tessariq/  \n",
			line:    ".tessariq/",
			want:    true,
		},
		{
			name:    "partial match is not a match",
			content: ".tessariq/runs/\n",
			line:    ".tessariq/",
			want:    false,
		},
		{
			name:    "no trailing newline",
			content: ".tessariq/",
			line:    ".tessariq/",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := containsLine(tt.content, tt.line)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRun_CreatesDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	err := Run(root)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(root, ".tessariq", "runs"))
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestRun_CreatesGitignoreWhenMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	err := Run(root)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	require.Contains(t, string(content), ".tessariq/\n")
}

func TestRun_AppendsWithoutDuplication(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	existing := "dist/\n*.out\n"
	err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(existing), 0o644)
	require.NoError(t, err)

	err = Run(root)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	require.Equal(t, existing+".tessariq/\n", string(content))
}

func TestRun_SkipsWhenAlreadyPresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	existing := "dist/\n.tessariq/\n*.out\n"
	err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(existing), 0o644)
	require.NoError(t, err)

	err = Run(root)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	require.Equal(t, existing, string(content))
}

func TestRun_HandlesFileWithoutTrailingNewline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	existing := "dist/\n*.out"
	err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(existing), 0o644)
	require.NoError(t, err)

	err = Run(root)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	require.Equal(t, existing+"\n.tessariq/\n", string(content))
}

func TestRun_IdempotentOnRerun(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	err := Run(root)
	require.NoError(t, err)

	first, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)

	err = Run(root)
	require.NoError(t, err)

	second, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)

	require.Equal(t, string(first), string(second))

	// Directory still exists
	info, err := os.Stat(filepath.Join(root, ".tessariq", "runs"))
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestRun_DirectoryPermissions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	err := Run(root)
	require.NoError(t, err)

	for _, rel := range []string{".tessariq", ".tessariq/runs"} {
		info, err := os.Stat(filepath.Join(root, rel))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
			"%s must be owner-only", rel)
	}
}

func TestRun_IdempotentPreservesSecurePermissions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	require.NoError(t, Run(root))
	require.NoError(t, Run(root))

	for _, rel := range []string{".tessariq", ".tessariq/runs"} {
		info, err := os.Stat(filepath.Join(root, rel))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
			"%s must remain owner-only after re-run", rel)
	}
}

func TestRun_TightensRelaxedPermissions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Pre-create directories with overly permissive mode.
	tessariqDir := filepath.Join(root, ".tessariq")
	runsDir := filepath.Join(tessariqDir, "runs")
	require.NoError(t, os.MkdirAll(runsDir, 0o755))

	require.NoError(t, Run(root))

	for _, rel := range []string{".tessariq", ".tessariq/runs"} {
		info, err := os.Stat(filepath.Join(root, rel))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
			"%s must be tightened to owner-only", rel)
	}
}

func TestRun_DoesNotCreateSpecsDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	err := Run(root)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(root, "specs"))
	require.True(t, os.IsNotExist(err), "specs/ must not be created by init")
}
