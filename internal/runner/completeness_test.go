package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	manifestDirect = `{"schema_version":1,"run_id":"test-run","task_path":"tasks/t.md","agent":"claude-code","base_sha":"abc123","workspace_mode":"worktree","resolved_egress_mode":"direct","container_name":"tessariq-test","created_at":"2026-01-01T00:00:00Z"}`
	manifestProxy  = `{"schema_version":1,"run_id":"test-run","task_path":"tasks/t.md","agent":"claude-code","base_sha":"abc123","workspace_mode":"worktree","resolved_egress_mode":"proxy","container_name":"tessariq-test","created_at":"2026-01-01T00:00:00Z"}`
)

// runtimeJSON builds a full-shape runtime.json fixture that satisfies the
// structured-field validation. When mode is non-empty it also records
// resolved_egress_mode; when empty the field is omitted, modelling a
// runtime.json that lacks the trusted egress mode.
func runtimeJSON(mode string) string {
	const base = `"schema_version":1,"image":"test","image_source":"custom","auth_mount_mode":"read-only","agent_config_mount":"disabled","agent_config_mount_status":"disabled"`
	if mode == "" {
		return "{" + base + "}"
	}
	return "{" + base + `,"resolved_egress_mode":"` + mode + `"}`
}

func writeBaseEvidence(t *testing.T, dir, manifestJSON string) {
	t.Helper()
	// Mirror the manifest's resolved egress mode into runtime.json so the
	// base fixture represents an intact (non-tampered) run. runtime.json is
	// the trusted source of resolved egress mode, so it must carry the field.
	writeBaseEvidenceWithRuntime(t, dir, manifestJSON, runtimeJSON(manifestEgressMode(manifestJSON)))
}

// manifestEgressMode extracts resolved_egress_mode from a manifest JSON
// string for test fixtures. Returns "" when the manifest is unparseable
// (e.g. the malformed-manifest case).
func manifestEgressMode(manifestJSON string) string {
	var m struct {
		ResolvedEgressMode string `json:"resolved_egress_mode"`
	}
	if err := json.Unmarshal([]byte(manifestJSON), &m); err != nil {
		return ""
	}
	return m.ResolvedEgressMode
}

func writeBaseEvidenceWithRuntime(t *testing.T, dir, manifestJSON, runtimeJSON string) {
	t.Helper()
	files := map[string]string{
		"manifest.json":  manifestJSON,
		"status.json":    `{"schema_version":1,"state":"success","started_at":"2026-01-01T00:00:00Z"}`,
		"agent.json":     `{"schema_version":1,"agent":"claude-code"}`,
		"runtime.json":   runtimeJSON,
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

func TestCheckEvidenceCompleteness_ProxyModeEmptyEventsJSONLFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\nallowlist_source: built_in\ndestinations:\n  - host: example.com\n    port: 443\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte{}, 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err, "0-byte egress.events.jsonl indicates truncation or extraction failure")
	require.ErrorContains(t, err, "egress.events.jsonl")
}

func TestCheckEvidenceCompleteness_ProxyModeZeroEventsSummaryPasses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"),
		[]byte("schema_version: 1\nallowlist_source: built_in\ndestinations:\n  - host: example.com\n    port: 443\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"),
		[]byte("{\"schema_version\":1,\"event_count\":0}\n"), 0o600))

	require.NoError(t, CheckEvidenceCompleteness(dir),
		"summary-line egress.events.jsonl means zero denied events, should pass")
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
		{"fractional schema_version on agent.json", "agent.json", `{"schema_version":1.9,"agent":"claude-code"}`, "schema_version"},
		{"fractional schema_version on runtime.json", "runtime.json", `{"schema_version":1.9,"image":"test","image_source":"custom","auth_mount_mode":"read-only","agent_config_mount":"disabled","agent_config_mount_status":"disabled"}`, "schema_version"},
		{"non-string agent field", "agent.json", `{"schema_version":1,"agent":42}`, "agent"},
		{"non-string runtime image field", "runtime.json", `{"schema_version":1,"image":42,"image_source":"custom","auth_mount_mode":"read-only","agent_config_mount":"disabled","agent_config_mount_status":"disabled"}`, "image"},
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

func TestCheckEvidenceCompleteness_EgressModeMismatchManifestDirectRuntimeProxy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidenceWithRuntime(t, dir, manifestDirect, runtimeJSON("proxy"))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEgressModeMismatch)
	require.ErrorContains(t, err, "tampered")
}

func TestCheckEvidenceCompleteness_EgressModeMismatchManifestProxyRuntimeDirect(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidenceWithRuntime(t, dir, manifestProxy, runtimeJSON("direct"))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEgressModeMismatch)
	require.ErrorContains(t, err, "tampered")
}

func TestCheckEvidenceCompleteness_BothAgreeDirectPasses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidenceWithRuntime(t, dir, manifestDirect, runtimeJSON("direct"))

	require.NoError(t, CheckEvidenceCompleteness(dir))
}

func TestCheckEvidenceCompleteness_BothAgreeProxyWithArtifactsPasses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidenceWithRuntime(t, dir, manifestProxy, runtimeJSON("proxy"))
	writeValidProxyEvidence(t, dir)

	require.NoError(t, CheckEvidenceCompleteness(dir))
}

func TestCheckEvidenceCompleteness_BothAgreeProxyMissingArtifactsFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidenceWithRuntime(t, dir, manifestProxy, runtimeJSON("proxy"))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "egress.compiled.yaml")
	require.ErrorContains(t, err, "egress.events.jsonl")
}

// A runtime.json that omits resolved_egress_mode must fail closed. Falling
// back to manifest.json would reopen the tamper path: an attacker can relabel
// a proxy run to "direct" in the manifest and drop the field from runtime.json
// (the file stays non-empty, so the file-presence check still passes), which
// would otherwise skip the required proxy evidence. The trusted resolved mode
// must come from runtime.json, never the mutable manifest alone.
func TestCheckEvidenceCompleteness_RuntimeMissingEgressModeFailsClosed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidenceWithRuntime(t, dir, manifestProxy, runtimeJSON(""))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte(`{"host":"x"}`+"\n"), 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "runtime.json")
	require.ErrorContains(t, err, "resolved_egress_mode")
}

// The exact suppression attack: proxy run relabeled to "direct" in the
// manifest with the runtime field dropped and no proxy artifacts present.
// Fail-closed on the missing runtime field blocks it before promote.
func TestCheckEvidenceCompleteness_DirectManifestEmptyRuntimeFailsClosed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidenceWithRuntime(t, dir, manifestDirect, runtimeJSON(""))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "runtime.json")
	require.ErrorContains(t, err, "resolved_egress_mode")
}
