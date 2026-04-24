package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	manifestDirect = `{"schema_version":1,"resolved_egress_mode":"direct"}`
	manifestProxy  = `{"schema_version":1,"resolved_egress_mode":"proxy"}`
)

func runtimeJSON(mode string) string {
	return `{"schema_version":1,"resolved_egress_mode":"` + mode + `"}`
}

func writeBaseEvidence(t *testing.T, dir, manifestJSON string) {
	t.Helper()
	writeBaseEvidenceWithRuntime(t, dir, manifestJSON, `{"schema_version":1}`)
}

func writeBaseEvidenceWithRuntime(t *testing.T, dir, manifestJSON, runtimeJSON string) {
	t.Helper()
	files := map[string]string{
		"manifest.json":  manifestJSON,
		"status.json":    `{"schema_version":1}`,
		"agent.json":     `{"schema_version":1}`,
		"runtime.json":   runtimeJSON,
		"task.md":        "# Task\n",
		"run.log":        "ok\n",
		"runner.log":     "ok\n",
		"workspace.json": `{"schema_version":1}`,
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte(`{"host":"x"}`+"\n"), 0o600))

	require.NoError(t, CheckEvidenceCompleteness(dir))
}

func TestCheckEvidenceCompleteness_ProxyModeMissingCompiledYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte(`{"host":"x"}`+"\n"), 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "egress.compiled.yaml")
	require.NotContains(t, err.Error(), "egress.events.jsonl")
}

func TestCheckEvidenceCompleteness_ProxyModeMissingEventsJSONL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\n"), 0o600))

	err := CheckEvidenceCompleteness(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "egress.events.jsonl")
	require.NotContains(t, err.Error(), "egress.compiled.yaml")
}

func TestCheckEvidenceCompleteness_ProxyModeEmptyEgressArtifact(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\n"), 0o600))
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte(`{"host":"x"}`+"\n"), 0o600))

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

func TestCheckEvidenceCompleteness_EmptyRuntimeModeFallsBackToManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidenceWithRuntime(t, dir, manifestProxy, `{"schema_version":1}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte(`{"host":"x"}`+"\n"), 0o600))

	require.NoError(t, CheckEvidenceCompleteness(dir))
}

func TestCheckEvidenceCompleteness_EmptyRuntimeModeDirectManifestSkipsProxy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidenceWithRuntime(t, dir, manifestDirect, `{"schema_version":1}`)

	require.NoError(t, CheckEvidenceCompleteness(dir))
}
