package runner

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignalState_SIGTERM(t *testing.T) {
	t.Parallel()
	require.Equal(t, StateKilled, SignalState(syscall.SIGTERM))
}

func TestSignalState_SIGINT(t *testing.T) {
	t.Parallel()
	require.Equal(t, StateInterrupted, SignalState(syscall.SIGINT))
}

func TestSignalState_UnknownDefaultsToKilled(t *testing.T) {
	t.Parallel()
	require.Equal(t, StateKilled, SignalState(os.Kill))
}
