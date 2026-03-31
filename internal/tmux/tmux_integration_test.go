//go:build integration

package tmux

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}
}

func TestNewSession_Integration_CreatesSession(t *testing.T) {
	t.Parallel()
	skipIfNoTmux(t)

	ctx := context.Background()
	name := "tessariq-test-create-" + t.Name()
	t.Cleanup(func() { _ = KillSession(ctx, name) })

	require.NoError(t, NewSession(ctx, name, nil))

	exists, err := HasSession(ctx, name)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestNewSession_Integration_DuplicateSessionFails(t *testing.T) {
	t.Parallel()
	skipIfNoTmux(t)

	ctx := context.Background()
	name := "tessariq-test-dup-" + t.Name()
	t.Cleanup(func() { _ = KillSession(ctx, name) })

	require.NoError(t, NewSession(ctx, name, nil))
	err := NewSession(ctx, name, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), name)
}

func TestHasSession_Integration_ReturnsFalseForMissing(t *testing.T) {
	t.Parallel()
	skipIfNoTmux(t)

	ctx := context.Background()
	exists, err := HasSession(ctx, "tessariq-test-nonexistent-session")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestKillSession_Integration_NoErrorOnMissing(t *testing.T) {
	t.Parallel()
	skipIfNoTmux(t)

	ctx := context.Background()
	require.NoError(t, KillSession(ctx, "tessariq-test-nonexistent-kill"))
}
