package attach

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
)

func TestResolveLiveRun_ReturnsLiveSession(t *testing.T) {
	t.Parallel()

	result, err := resolveLiveRun(context.Background(), "/repo", "last", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			require.Equal(t, filepath.Join("/repo", ".tessariq", "runs"), runsDir)
			require.Equal(t, "last", ref)
			return run.IndexEntry{RunID: "RUN123", EvidencePath: filepath.Join(".tessariq", "runs", "RUN123")}, nil
		},
		readStatus: func(evidenceDir string) (runner.Status, error) {
			require.Equal(t, filepath.Join("/repo", ".tessariq", "runs", "RUN123"), evidenceDir)
			return runner.Status{State: runner.StateRunning}, nil
		},
		hasSession: func(ctx context.Context, sessionName string) (bool, error) {
			require.Equal(t, run.SessionName("RUN123"), sessionName)
			return true, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, Result{
		RunID:        "RUN123",
		SessionName:  run.SessionName("RUN123"),
		EvidencePath: filepath.Join("/repo", ".tessariq", "runs", "RUN123"),
	}, result)
}

func TestResolveLiveRun_FinishedRunReturnsNotLiveError(t *testing.T) {
	t.Parallel()

	_, err := resolveLiveRun(context.Background(), "/repo", "last", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			return run.IndexEntry{RunID: "RUN123", EvidencePath: filepath.Join(".tessariq", "runs", "RUN123")}, nil
		},
		readStatus: func(evidenceDir string) (runner.Status, error) {
			return runner.Status{State: runner.StateSuccess}, nil
		},
		hasSession: func(ctx context.Context, sessionName string) (bool, error) {
			return true, nil
		},
	})
	require.ErrorIs(t, err, ErrRunNotLive)
	require.ErrorContains(t, err, "run RUN123 is not live")
	require.ErrorContains(t, err, "evidence path: /repo/.tessariq/runs/RUN123")
	require.ErrorContains(t, err, "state success")
}

func TestResolveLiveRun_UnknownRunReturnsNotLiveError(t *testing.T) {
	t.Parallel()

	_, err := resolveLiveRun(context.Background(), "/repo", "last", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			return run.IndexEntry{}, run.ErrRunNotFound
		},
	})
	require.ErrorIs(t, err, ErrRunNotLive)
	require.ErrorContains(t, err, "run last is not live")
	require.ErrorContains(t, err, "no matching run found")
}

func TestResolveLiveRun_MissingSessionReturnsNotLiveError(t *testing.T) {
	t.Parallel()

	_, err := resolveLiveRun(context.Background(), "/repo", "RUN123", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			return run.IndexEntry{RunID: "RUN123", EvidencePath: filepath.Join(".tessariq", "runs", "RUN123")}, nil
		},
		readStatus: func(evidenceDir string) (runner.Status, error) {
			return runner.Status{State: runner.StateRunning}, nil
		},
		hasSession: func(ctx context.Context, sessionName string) (bool, error) {
			return false, nil
		},
	})
	require.ErrorIs(t, err, ErrRunNotLive)
	require.ErrorContains(t, err, "run RUN123 is not live")
	require.ErrorContains(t, err, "evidence path: /repo/.tessariq/runs/RUN123")
	require.ErrorContains(t, err, "no live tmux session")
}

func TestResolveLiveRun_SessionCheckErrorReturned(t *testing.T) {
	t.Parallel()

	_, err := resolveLiveRun(context.Background(), "/repo", "RUN123", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			return run.IndexEntry{RunID: "RUN123", EvidencePath: filepath.Join(".tessariq", "runs", "RUN123")}, nil
		},
		readStatus: func(evidenceDir string) (runner.Status, error) {
			return runner.Status{State: runner.StateRunning}, nil
		},
		hasSession: func(ctx context.Context, sessionName string) (bool, error) {
			return false, errors.New("tmux exploded")
		},
	})
	require.ErrorContains(t, err, "check tmux session for run RUN123")
	require.ErrorContains(t, err, "tmux exploded")
}
