package tmux

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrTmuxNotAvailable_IsStableError(t *testing.T) {
	t.Parallel()

	require.True(t, errors.Is(ErrTmuxNotAvailable, ErrTmuxNotAvailable))
}

func TestErrTmuxNotAvailable_ContainsGuidance(t *testing.T) {
	t.Parallel()

	require.Contains(t, ErrTmuxNotAvailable.Error(), "install tmux")
}

func TestAvailable_ReturnsErrWhenNotOnPath(t *testing.T) {
	// Cannot use t.Parallel with t.Setenv.
	t.Setenv("PATH", "")

	err := Available()
	require.ErrorIs(t, err, ErrTmuxNotAvailable)
}

func TestNewSessionArgs_WithoutCommand(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"new-session", "-d", "-s", "sess"}, newSessionArgs("sess", nil))
}

func TestNewSessionArgs_WithCommand(t *testing.T) {
	t.Parallel()

	args := newSessionArgs("sess", []string{"tail", "-n", "+1", "-f", "/tmp/run.log"})
	require.Equal(t, []string{"new-session", "-d", "-s", "sess", "'tail' '-n' '+1' '-f' '/tmp/run.log'"}, args)
}
