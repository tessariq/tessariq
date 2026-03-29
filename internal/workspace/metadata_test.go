package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildMetadata_Constants(t *testing.T) {
	t.Parallel()

	m := BuildMetadata("abc123def456", "/home/user/.tessariq/worktrees/repo-12345678/01ARZ3ND")

	require.Equal(t, 1, m.SchemaVersion)
	require.Equal(t, "worktree", m.WorkspaceMode)
	require.Equal(t, "rw", m.RepoMountMode)
	require.True(t, m.RepoClean)
	require.Equal(t, "strong", m.Reproducibility)
}

func TestBuildMetadata_VariableFields(t *testing.T) {
	t.Parallel()

	baseSHA := "abc123def456"
	wsPath := "/home/user/.tessariq/worktrees/repo-12345678/01ARZ3ND"

	m := BuildMetadata(baseSHA, wsPath)

	require.Equal(t, baseSHA, m.BaseSHA)
	require.Equal(t, wsPath, m.WorkspacePath)
}

func TestBuildMetadata_ExactlySevenFields(t *testing.T) {
	t.Parallel()

	m := BuildMetadata("sha", "/path")

	data, err := json.Marshal(m)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	expectedKeys := map[string]bool{
		"schema_version":  true,
		"workspace_mode":  true,
		"base_sha":        true,
		"workspace_path":  true,
		"repo_mount_mode": true,
		"repo_clean":      true,
		"reproducibility": true,
	}

	for k := range raw {
		require.True(t, expectedKeys[k], "unexpected key in workspace metadata: %s", k)
	}
	require.Len(t, raw, len(expectedKeys), "workspace metadata should have exactly %d keys", len(expectedKeys))
}

func TestWriteMetadata_CreatesDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	m := BuildMetadata("abc123", "/some/path")

	require.NoError(t, WriteMetadata(dir, m))

	_, err := os.Stat(dir)
	require.NoError(t, err)
}

func TestWriteMetadata_WritesValidJSON(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	m := BuildMetadata("abc123", "/some/path")

	require.NoError(t, WriteMetadata(dir, m))

	data, err := os.ReadFile(filepath.Join(dir, "workspace.json"))
	require.NoError(t, err)

	var parsed Metadata
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, m, parsed)
}
