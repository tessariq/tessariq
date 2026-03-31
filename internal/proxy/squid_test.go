package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSquidContainerName(t *testing.T) {
	t.Parallel()

	got := SquidContainerName("abc123")
	require.Equal(t, "tessariq-squid-abc123", got)
}

func TestSquidAddress(t *testing.T) {
	t.Parallel()

	got := SquidAddress("tessariq-squid-abc123")
	require.Equal(t, "http://tessariq-squid-abc123:3128", got)
}

func TestSquidAddress_CustomNames(t *testing.T) {
	t.Parallel()

	a := SquidAddress("squid-alpha")
	b := SquidAddress("squid-beta")

	require.NotEqual(t, a, b)
	require.Equal(t, "http://squid-alpha:3128", a)
	require.Equal(t, "http://squid-beta:3128", b)
}
