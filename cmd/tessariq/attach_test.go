package main

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	intattach "github.com/tessariq/tessariq/internal/attach"
)

func TestNewAttachCmd_IncludesDetachGuidance(t *testing.T) {
	cmd := newAttachCmd()
	require.Contains(t, cmd.Long, "Ctrl-b d")
}

func TestNewAttachCmd_MissingTmuxReturnsActionableGuidance(t *testing.T) {
	originalCheck := checkAttachPrereq
	t.Cleanup(func() { checkAttachPrereq = originalCheck })
	checkAttachPrereq = func() error {
		return errors.New("required host prerequisite \"tmux\" is missing or unavailable; install or enable tmux, then retry")
	}

	cmd := newAttachCmd()
	cmd.SetArgs([]string{"last"})
	err := cmd.Execute()
	require.EqualError(t, err, "required host prerequisite \"tmux\" is missing or unavailable; install or enable tmux, then retry")
}

func TestNewAttachCmd_MissingGitReturnsActionableGuidance(t *testing.T) {
	originalCheck := checkAttachPrereq
	t.Cleanup(func() { checkAttachPrereq = originalCheck })
	checkAttachPrereq = func() error {
		return errors.New("required host prerequisite \"git\" is missing or unavailable; install or enable git, then retry")
	}

	cmd := newAttachCmd()
	cmd.SetArgs([]string{"last"})
	err := cmd.Execute()
	require.EqualError(t, err, "required host prerequisite \"git\" is missing or unavailable; install or enable git, then retry")
}

func TestNewAttachCmd_AttachesResolvedSession(t *testing.T) {
	originalCheck := checkAttachPrereq
	originalRepoRoot := attachRepoRoot
	originalResolve := resolveAttachRun
	originalAttach := attachToSession
	t.Cleanup(func() {
		checkAttachPrereq = originalCheck
		attachRepoRoot = originalRepoRoot
		resolveAttachRun = originalResolve
		attachToSession = originalAttach
	})

	checkAttachPrereq = func() error { return nil }
	attachRepoRoot = func(cmd *cobra.Command) (string, error) {
		return "/repo", nil
	}
	resolveAttachRun = func(ctx context.Context, repoRoot, ref string) (intattach.Result, error) {
		require.Equal(t, "/repo", repoRoot)
		require.Equal(t, "last", ref)
		return intattach.Result{SessionName: "tessariq-RUN123"}, nil
	}
	attached := false
	attachToSession = func(ctx context.Context, sessionName string) error {
		attached = true
		require.Equal(t, "tessariq-RUN123", sessionName)
		return nil
	}

	cmd := newAttachCmd()
	cmd.SetArgs([]string{"last"})
	require.NoError(t, cmd.Execute())
	require.True(t, attached)
}
