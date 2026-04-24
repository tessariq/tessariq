package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	manifestDirect = `{"schema_version":1,"run_id":"test-run","task_path":"tasks/t.md","agent":"claude-code","base_sha":"abc123","workspace_mode":"worktree","resolved_egress_mode":"direct","container_name":"tessariq-test","created_at":"2026-01-01T00:00:00Z"}`
	manifestProxy  = `{"schema_version":1,"run_id":"test-run","task_path":"tasks/t.md","agent":"claude-code","base_sha":"abc123","workspace_mode":"worktree","resolved_egress_mode":"proxy","container_name":"tessariq-test","created_at":"2026-01-01T00:00:00Z"}`
)

func writeBaseEvidence(t *testing.T, dir, manifestJSON string) {
	t.Helper()
	files := map[string]string{
		"manifest.json":  manifestJSON,
		"status.json":    `{"schema_version":1,"state":"success","started_at":"2026-01-01T00:00:00Z"}`,
		"agent.json":     `{"schema_version":1,"agent":"claude-code"}`,
		"runtime.json":   `{"schema_version":1,"image":"test","image_source":"custom","auth_mount_mode":"read-only","agent_config_mount":"disabled","agent_config_mount_status":"disabled"}`,
		"task.md":        "# Task\n",
		"run.log":        "ok\n",
		"runner.log":     "ok\n",
		"workspace.json": `{"schema_version":1,"workspace_mode":"worktree","base_sha":"abc123","workspace_path":"/tmp/ws","repo_mount_mode":"rw","reproducibility":"strong"}`,
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
}

func TestCheckEvidenceCompleteness_AllPresent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestDirect)

	require.NoError(t, CheckEvidenceCompleteness(dir))
}

func TestCheckEvidenceCompleteness_MissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestDirect)
	require.NoError(t, os.Remove(filepath.Join(dir, "status.json")))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "status.json")
}

func TestCheckEvidenceCompleteness_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestDirect)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run.log"), []byte{}, 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "run.log (empty)")
}

func TestCheckEvidenceCompleteness_MultipleMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifestDirect), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run.log"), []byte("ok\n"), 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "status.json")
	require.ErrorContains(t, err, "workspace.json")
}

func TestCheckEvidenceCompleteness_DirectModeNoEgressArtifactsPasses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestDirect)

	require.NoError(t, CheckEvidenceCompleteness(dir))
}

func TestCheckEvidenceCompleteness_ProxyModeBothArtifactsPresentPasses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	writeValidProxyEvidence(t, dir)

	require.NoError(t, CheckEvidenceCompleteness(dir))
}

func TestCheckEvidenceCompleteness_ProxyModeMissingCompiledYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte(`{"timestamp":"2026-01-01T00:00:00Z","host":"x","port":443,"action":"blocked","reason":"not_in_allowlist","squid_result":"TCP_DENIED/403"}`+"\n"), 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "egress.compiled.yaml")
	require.NotContains(t, err.Error(), "egress.events.jsonl")
}

func TestCheckEvidenceCompleteness_ProxyModeMissingEventsJSONL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\nallowlist_source: built_in\ndestinations:\n  - host: example.com\n    port: 443\n"), 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "egress.events.jsonl")
	require.NotContains(t, err.Error(), "egress.compiled.yaml")
}

func TestCheckEvidenceCompleteness_ProxyModeEmptyEgressArtifact(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\nallowlist_source: built_in\ndestinations:\n  - host: example.com\n    port: 443\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte{}, 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "egress.events.jsonl (empty)")
}

func TestCheckEvidenceCompleteness_ProxyModeBothMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "egress.compiled.yaml")
	require.ErrorContains(t, err, "egress.events.jsonl")
}

func TestCheckEvidenceCompleteness_MalformedManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, "not-json")

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "manifest")
}

func TestCheckEvidenceCompleteness_MalformedStructuredArtifacts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		artifact string
		content  string
		wantErr  string
	}{
		{"malformed status.json", "status.json", "not-json", "status.json"},
		{"malformed agent.json", "agent.json", "not-json", "agent.json"},
		{"malformed runtime.json", "runtime.json", "not-json", "runtime.json"},
		{"malformed workspace.json", "workspace.json", "not-json", "workspace.json"},
		{"status missing state", "status.json", `{"schema_version":1,"started_at":"2026-01-01T00:00:00Z"}`, "state"},
		{"status missing started_at", "status.json", `{"schema_version":1,"state":"success"}`, "started_at"},
		{"agent missing agent field", "agent.json", `{"schema_version":1}`, "agent"},
		{"runtime missing image", "runtime.json", `{"schema_version":1,"image_source":"custom","auth_mount_mode":"read-only","agent_config_mount":"disabled","agent_config_mount_status":"disabled"}`, "image"},
		{"workspace missing base_sha", "workspace.json", `{"schema_version":1,"workspace_mode":"worktree","workspace_path":"/tmp/ws","repo_mount_mode":"rw","reproducibility":"strong"}`, "base_sha"},
		{"manifest missing run_id", "manifest.json", `{"schema_version":1,"task_path":"t","agent":"a","base_sha":"b","workspace_mode":"w","resolved_egress_mode":"direct","container_name":"c","created_at":"2026-01-01T00:00:00Z"}`, "run_id"},
		{"bad schema_version on agent.json", "agent.json", `{"schema_version":2,"agent":"claude-code"}`, "schema_version"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeBaseEvidence(t, dir, manifestDirect)
			require.NoError(t, os.WriteFile(filepath.Join(dir, tc.artifact), []byte(tc.content), 0o600))

			err := CheckEvidenceCompleteness(dir)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestCheckEvidenceCompleteness_MalformedProxyArtifacts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		yaml    string
		jsonl   string
		wantErr string
	}{
		{
			"malformed compiled yaml",
			"not: [valid: yaml: {{",
			`{"timestamp":"t","host":"x","port":443,"action":"blocked","reason":"r","squid_result":"s"}` + "\n",
			"egress.compiled.yaml",
		},
		{
			"compiled yaml missing allowlist_source",
			"schema_version: 1\ndestinations:\n  - host: x\n    port: 443\n",
			`{"timestamp":"t","host":"x","port":443,"action":"blocked","reason":"r","squid_result":"s"}` + "\n",
			"allowlist_source",
		},
		{
			"compiled yaml empty destinations",
			"schema_version: 1\nallowlist_source: built_in\ndestinations: []\n",
			`{"timestamp":"t","host":"x","port":443,"action":"blocked","reason":"r","squid_result":"s"}` + "\n",
			"destinations",
		},
		{
			"malformed events jsonl",
			"schema_version: 1\nallowlist_source: built_in\ndestinations:\n  - host: x\n    port: 443\n",
			"not-json\n",
			"egress.events.jsonl",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeBaseEvidence(t, dir, manifestProxy)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte(tc.yaml), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte(tc.jsonl), 0o600))

			err := CheckEvidenceCompleteness(dir)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func writeValidProxyEvidence(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"),
		[]byte("schema_version: 1\nallowlist_source: built_in\ndestinations:\n  - host: example.com\n    port: 443\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"),
		[]byte(`{"timestamp":"2026-01-01T00:00:00Z","host":"blocked.example.com","port":443,"action":"blocked","reason":"not_in_allowlist","squid_result":"TCP_DENIED/403"}`+"\n"), 0o600))
}
