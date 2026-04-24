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

func writeBaseEvidence(t *testing.T, dir, manifestJSON string) {
	t.Helper()
	files := map[string]string{
		"manifest.json":  manifestJSON,
		"status.json":    `{"schema_version":1}`,
		"agent.json":     `{"schema_version":1}`,
		"runtime.json":   `{"schema_version":1}`,
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

func TestCheckEvidenceCompleteness_ProxyModeEmptyEventsJSONLPasses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeBaseEvidence(t, dir, manifestProxy)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.compiled.yaml"), []byte("schema_version: 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "egress.events.jsonl"), []byte{}, 0o600))

	require.NoError(t, CheckEvidenceCompleteness(dir), "0-byte egress.events.jsonl means no blocked events, not extraction failure")
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
