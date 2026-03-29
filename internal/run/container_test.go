package run

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainerName_DeterministicPrefix(t *testing.T) {
	t.Parallel()

	name := ContainerName("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Equal(t, "tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV", name)
}

func TestContainerName_AlwaysHasPrefix(t *testing.T) {
	t.Parallel()

	name := ContainerName("01JNQHZBX0000000000000000")
	require.Contains(t, name, "tessariq-")
	require.Equal(t, "tessariq-01JNQHZBX0000000000000000", name)
}

func TestContainerName_EmptyRunID(t *testing.T) {
	t.Parallel()

	name := ContainerName("")
	require.Equal(t, "tessariq-", name)
}
