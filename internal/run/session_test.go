package run

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionName_Deterministic(t *testing.T) {
	t.Parallel()

	name := SessionName("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Equal(t, "tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV", name)
}

func TestSessionName_HasPrefix(t *testing.T) {
	t.Parallel()

	name := SessionName("ANYID")
	require.Contains(t, name, "tessariq-")
}

func TestSessionName_EqualsContainerName(t *testing.T) {
	t.Parallel()

	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	require.Equal(t, ContainerName(runID), SessionName(runID))
}

func TestSessionName_EmptyRunID(t *testing.T) {
	t.Parallel()

	require.Equal(t, "tessariq-", SessionName(""))
}
