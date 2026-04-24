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

func TestMetadata_Validate(t *testing.T) {
	t.Parallel()

	valid := BuildMetadata("abc123", "/some/path")
	require.NoError(t, valid.Validate())

	cases := []struct {
		name    string
		mutate  func(*Metadata)
		wantErr string
	}{
		{"bad schema_version", func(m *Metadata) { m.SchemaVersion = 0 }, "schema_version"},
		{"missing workspace_mode", func(m *Metadata) { m.WorkspaceMode = "" }, "workspace_mode"},
		{"missing base_sha", func(m *Metadata) { m.BaseSHA = "" }, "base_sha"},
		{"missing workspace_path", func(m *Metadata) { m.WorkspacePath = "" }, "workspace_path"},
		{"missing repo_mount_mode", func(m *Metadata) { m.RepoMountMode = "" }, "repo_mount_mode"},
		{"missing reproducibility", func(m *Metadata) { m.Reproducibility = "" }, "reproducibility"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := valid
			tc.mutate(&m)
			err := m.Validate()
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestReadMetadata_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	original := BuildMetadata("abc123", "/some/path")
	require.NoError(t, WriteMetadata(dir, original))

	got, err := ReadMetadata(dir)
	require.NoError(t, err)
	require.Equal(t, original, got)
}

func TestReadMetadata_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := ReadMetadata(t.TempDir())
	require.Error(t, err)
}

func TestReadMetadata_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "workspace.json"), []byte("not-json"), 0o600))

	_, err := ReadMetadata(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "parse")
}

func TestWriteMetadata_DirectoryPermissions(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	m := BuildMetadata("abc123", "/some/path")

	require.NoError(t, WriteMetadata(dir, m))

	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
		"evidence directory must be owner-only")
}

func TestWriteMetadata_FilePermissions(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	m := BuildMetadata("abc123", "/some/path")

	require.NoError(t, WriteMetadata(dir, m))

	info, err := os.Stat(filepath.Join(dir, "workspace.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"evidence file must be owner-only read/write")
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
