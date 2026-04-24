package proxy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTopology_SquidImageDefault(t *testing.T) {
	t.Parallel()

	topo := &Topology{
		RunID:           "test-run-1",
		EvidenceDir:     t.TempDir(),
		Destinations:    []string{"api.example.com:443"},
		AllowlistSource: "cli",
	}

	require.Equal(t, "", topo.SquidImage, "SquidImage should start empty")
	// The effective image used during Setup should be DefaultSquidImage
	// when SquidImage is empty. We verify the field is empty and the
	// constant is digest-pinned.
	require.Contains(t, DefaultSquidImage, "@sha256:", "DefaultSquidImage must be pinned by digest")
}

func TestProxyEnv_Fields(t *testing.T) {
	t.Parallel()

	env := &ProxyEnv{
		ProxyAddr:   "http://tessariq-squid-abc:3128",
		NetworkName: "tessariq-net-abc",
	}

	require.Equal(t, "http://tessariq-squid-abc:3128", env.ProxyAddr)
	require.Equal(t, "tessariq-net-abc", env.NetworkName)
}

func TestWriteExtractedEvidence_Success_WritesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logData := []byte("1714000000.000      1 192.168.1.1 TCP_DENIED/403 0 CONNECT blocked.example.com:443 - HIER_NONE/- -\n")

	err := WriteExtractedEvidence(dir, logData, 10*1024*1024)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "egress.events.jsonl"))
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0), "events file should be non-empty")

	info, err = os.Stat(filepath.Join(dir, "squid.log"))
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0), "squid log should be non-empty")
}

func TestWriteExtractedEvidence_EmptyLog_CreatesEmptyEventsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := WriteExtractedEvidence(dir, []byte{}, 10*1024*1024)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "egress.events.jsonl"))
	require.NoError(t, err)
	require.Equal(t, int64(0), info.Size(), "events file should be empty (no blocked events)")

	_, err = os.Stat(filepath.Join(dir, "squid.log"))
	require.NoError(t, err, "squid.log should still be created")
}

func TestWriteExtractedEvidence_BadEvidenceDir_ReturnsError(t *testing.T) {
	t.Parallel()

	err := WriteExtractedEvidence("/nonexistent/path", []byte{}, 10*1024*1024)
	require.Error(t, err)

	_, statErr := os.Stat("/nonexistent/path/egress.events.jsonl")
	require.True(t, os.IsNotExist(statErr), "no evidence files should be created on error")
}

func TestWriteExtractedEvidence_CopySquidLogFailure_RollsBackEventsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logData := []byte("1714000000.000      1 192.168.1.1 TCP_DENIED/403 0 CONNECT blocked.example.com:443 - HIER_NONE/- -\n")

	// Make squid.log path unwritable by creating a directory with that name.
	require.NoError(t, os.Mkdir(filepath.Join(dir, "squid.log"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "squid.log.tmp"), 0o755))

	err := WriteExtractedEvidence(dir, logData, 10*1024*1024)
	require.Error(t, err)
	require.Contains(t, err.Error(), "write squid log")

	_, statErr := os.Stat(filepath.Join(dir, "egress.events.jsonl"))
	require.True(t, os.IsNotExist(statErr),
		"egress.events.jsonl must be rolled back when CopySquidLog fails")
}

func TestTopology_TeardownIdempotent(t *testing.T) {
	t.Parallel()

	topo := &Topology{
		RunID:       "test-run-2",
		EvidenceDir: t.TempDir(),
		tornDown:    true,
	}

	err := topo.Teardown(context.Background())
	require.NoError(t, err, "teardown with tornDown=true should return nil without error")
}
