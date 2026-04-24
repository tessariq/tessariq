package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildManifestSeed_RequiredFields(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)

	m := BuildManifestSeed(cfg, runID, "Example Task", "abc123def456", "built_in", now)

	require.Equal(t, 1, m.SchemaVersion)
	require.Equal(t, "01ARZ3NDEKTSV4RRFFQ69G5FAV", m.RunID)
	require.Equal(t, "specs/example.md", m.TaskPath)
	require.Equal(t, "Example Task", m.TaskTitle)
	require.Equal(t, "claude-code", m.Agent)
	require.Equal(t, "abc123def456", m.BaseSHA)
	require.Equal(t, "worktree", m.WorkspaceMode)
	require.Equal(t, "auto", m.RequestedEgressMode)
	require.Equal(t, "proxy", m.ResolvedEgressMode)
	require.Equal(t, "built_in", m.AllowlistSource)
	require.Equal(t, "2026-03-29T12:00:00Z", m.CreatedAt)
}

func TestBuildManifestSeed_ExactlyTwelveFields(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Now()

	m := BuildManifestSeed(cfg, runID, "Task Title", "sha256", "built_in", now)

	data, err := json.Marshal(m)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	expectedKeys := map[string]bool{
		"schema_version":        true,
		"run_id":                true,
		"task_path":             true,
		"task_title":            true,
		"agent":                 true,
		"base_sha":              true,
		"workspace_mode":        true,
		"requested_egress_mode": true,
		"resolved_egress_mode":  true,
		"allowlist_source":      true,
		"container_name":        true,
		"created_at":            true,
	}

	for k := range raw {
		require.True(t, expectedKeys[k], "unexpected key in manifest: %s", k)
	}
	require.Len(t, raw, len(expectedKeys), "manifest should have exactly %d keys", len(expectedKeys))
}

func TestBuildManifestSeed_ContainerName(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Now()

	m := BuildManifestSeed(cfg, runID, "Task", "sha", "built_in", now)
	require.Equal(t, "tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV", m.ContainerName)
}

func TestBuildManifestSeed_UsesResolveEgress(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	cfg.UnsafeEgress = true
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Now()

	m := BuildManifestSeed(cfg, runID, "Task", "sha", "built_in", now)
	require.Equal(t, "open", m.RequestedEgressMode)
	require.Equal(t, "open", m.ResolvedEgressMode)
}

func TestBuildManifestSeed_CreatedAtFormat(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC)

	m := BuildManifestSeed(cfg, runID, "Task", "sha", "built_in", now)
	require.Equal(t, "2026-01-27T12:00:00Z", m.CreatedAt)
}

func TestBuildManifestSeed_RunIDFormat(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Now()

	m := BuildManifestSeed(cfg, runID, "Task", "sha", "built_in", now)
	require.True(t, IsValidRunID(m.RunID))
}

func TestBuildManifestSeed_AutoResolvesToProxy(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Now()

	m := BuildManifestSeed(cfg, runID, "Task", "sha", "built_in", now)
	require.Equal(t, "auto", m.RequestedEgressMode)
	require.Equal(t, "proxy", m.ResolvedEgressMode)
}

func TestBuildManifestSeed_ExplicitEgressNone(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	cfg.Egress = "none"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Now()

	m := BuildManifestSeed(cfg, runID, "Task", "sha", "built_in", now)
	require.Equal(t, "none", m.RequestedEgressMode)
	require.Equal(t, "none", m.ResolvedEgressMode)
}

func TestBuildManifestSeed_AllowlistSourceCLI(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	cfg.EgressAllow = []string{"api.openai.com:443"}
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Now()

	m := BuildManifestSeed(cfg, runID, "Task", "sha", "cli", now)
	require.Equal(t, "cli", m.AllowlistSource)
}

func TestBuildManifestSeed_AllowlistSourceBuiltIn(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Now()

	m := BuildManifestSeed(cfg, runID, "Task", "sha", "built_in", now)
	require.Equal(t, "built_in", m.AllowlistSource)
}

func TestBuildManifestSeed_AllowlistSourceUserConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	now := time.Now()

	m := BuildManifestSeed(cfg, runID, "Task", "sha", "user_config", now)
	require.Equal(t, "user_config", m.AllowlistSource)
}

func TestManifest_Validate(t *testing.T) {
	t.Parallel()

	valid := Manifest{
		SchemaVersion:      1,
		RunID:              "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		TaskPath:           "tasks/t.md",
		Agent:              "claude-code",
		BaseSHA:            "abc123",
		WorkspaceMode:      "worktree",
		ResolvedEgressMode: "direct",
		ContainerName:      "tessariq-test",
		CreatedAt:          "2026-01-01T00:00:00Z",
	}

	require.NoError(t, valid.Validate())

	cases := []struct {
		name    string
		mutate  func(*Manifest)
		wantErr string
	}{
		{"bad schema_version", func(m *Manifest) { m.SchemaVersion = 0 }, "schema_version"},
		{"missing run_id", func(m *Manifest) { m.RunID = "" }, "run_id"},
		{"missing task_path", func(m *Manifest) { m.TaskPath = "" }, "task_path"},
		{"missing agent", func(m *Manifest) { m.Agent = "" }, "agent"},
		{"missing base_sha", func(m *Manifest) { m.BaseSHA = "" }, "base_sha"},
		{"missing workspace_mode", func(m *Manifest) { m.WorkspaceMode = "" }, "workspace_mode"},
		{"missing resolved_egress_mode", func(m *Manifest) { m.ResolvedEgressMode = "" }, "resolved_egress_mode"},
		{"missing container_name", func(m *Manifest) { m.ContainerName = "" }, "container_name"},
		{"missing created_at", func(m *Manifest) { m.CreatedAt = "" }, "created_at"},
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

func TestWriteManifest_CreatesDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	m := Manifest{
		SchemaVersion:       1,
		RunID:               "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		TaskPath:            "specs/task.md",
		Agent:               "claude-code",
		RequestedEgressMode: "auto",
		ResolvedEgressMode:  "proxy",
		AllowlistSource:     "built_in",
		CreatedAt:           "2026-01-27T12:00:00Z",
	}

	require.NoError(t, WriteManifest(dir, m))

	_, err := os.Stat(dir)
	require.NoError(t, err)
}

func TestWriteManifest_WritesValidJSON(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	m := Manifest{
		SchemaVersion:       1,
		RunID:               "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		TaskPath:            "specs/task.md",
		Agent:               "claude-code",
		RequestedEgressMode: "auto",
		ResolvedEgressMode:  "proxy",
		AllowlistSource:     "built_in",
		CreatedAt:           "2026-01-27T12:00:00Z",
	}

	require.NoError(t, WriteManifest(dir, m))

	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err)

	var parsed Manifest
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, m, parsed)
}

func TestWriteManifest_DirectoryPermissions(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	m := Manifest{SchemaVersion: 1, RunID: "test"}

	require.NoError(t, WriteManifest(dir, m))

	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
		"evidence directory must be owner-only")
}

func TestWriteManifest_FilePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := Manifest{SchemaVersion: 1, RunID: "test"}

	require.NoError(t, WriteManifest(dir, m))

	info, err := os.Stat(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"evidence file must be owner-only read/write")
}

func TestWriteManifest_NoTempFileAfterSuccess(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "evidence")
	m := Manifest{SchemaVersion: 1, RunID: "test"}

	require.NoError(t, WriteManifest(dir, m))

	_, err := os.Stat(filepath.Join(dir, "manifest.json.tmp"))
	require.True(t, os.IsNotExist(err), "temp file must not remain after successful write")
}

func TestWriteManifest_OverwritesExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	original := Manifest{SchemaVersion: 1, RunID: "original"}
	require.NoError(t, WriteManifest(dir, original))

	updated := Manifest{SchemaVersion: 1, RunID: "updated"}
	require.NoError(t, WriteManifest(dir, updated))

	got, err := ReadManifest(dir)
	require.NoError(t, err)
	require.Equal(t, "updated", got.RunID)
}

func TestBootstrapManifest_Integration(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tessariq", "runs"), 0o755))

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/task.md"
	now := time.Now()

	runID, dir, err := BootstrapManifest(root, cfg, "Task Title", "abc123", "built_in", now)
	require.NoError(t, err)
	require.True(t, IsValidRunID(runID))
	require.Contains(t, dir, runID)

	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err)

	var parsed Manifest
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Equal(t, runID, parsed.RunID)
	require.Equal(t, "specs/task.md", parsed.TaskPath)
	require.Equal(t, "Task Title", parsed.TaskTitle)
	require.Equal(t, "claude-code", parsed.Agent)
	require.Equal(t, "abc123", parsed.BaseSHA)
	require.Equal(t, "worktree", parsed.WorkspaceMode)
	require.Equal(t, "auto", parsed.RequestedEgressMode)
	require.Equal(t, "proxy", parsed.ResolvedEgressMode)
	require.Equal(t, "built_in", parsed.AllowlistSource)
}
