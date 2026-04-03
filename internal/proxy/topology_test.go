package proxy

import (
	"context"
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
