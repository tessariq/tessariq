package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetworkName(t *testing.T) {
	t.Parallel()

	got := NetworkName("abc123")
	require.Equal(t, "tessariq-net-abc123", got)
}

func TestNetworkName_DifferentIDs(t *testing.T) {
	t.Parallel()

	a := NetworkName("run-1")
	b := NetworkName("run-2")
	require.NotEqual(t, a, b)
	require.Equal(t, "tessariq-net-run-1", a)
	require.Equal(t, "tessariq-net-run-2", b)
}
