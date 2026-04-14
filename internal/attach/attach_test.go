package attach

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
)

// setupAttachRepo creates a real repository root with an evidence directory
// for the given run ID so filepath.EvalSymlinks containment checks can resolve.
// It returns the repository root and the absolute path to the evidence directory.
func setupAttachRepo(t *testing.T, runID string) (string, string) {
	t.Helper()
	rootDir := t.TempDir()
	root, err := filepath.EvalSymlinks(rootDir)
	require.NoError(t, err)
	evidenceDir := filepath.Join(root, ".tessariq", "runs", runID)
	require.NoError(t, os.MkdirAll(evidenceDir, 0o755))
	return root, evidenceDir
}

func TestResolveLiveRun_ReturnsLiveSession(t *testing.T) {
	t.Parallel()

	root, evidenceDir := setupAttachRepo(t, "RUN123")
	result, err := resolveLiveRun(context.Background(), root, "last", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			require.Equal(t, filepath.Join(root, ".tessariq", "runs"), runsDir)
			require.Equal(t, "last", ref)
			return run.IndexEntry{RunID: "RUN123", EvidencePath: filepath.Join(".tessariq", "runs", "RUN123")}, nil
		},
		readStatus: func(got string) (runner.Status, error) {
			require.Equal(t, evidenceDir, got)
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
		EvidencePath: evidenceDir,
	}, result)
}

func TestResolveLiveRun_FinishedRunReturnsNotLiveError(t *testing.T) {
	t.Parallel()

	root, evidenceDir := setupAttachRepo(t, "RUN123")
	_, err := resolveLiveRun(context.Background(), root, "last", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			return run.IndexEntry{RunID: "RUN123", EvidencePath: filepath.Join(".tessariq", "runs", "RUN123")}, nil
		},
		readStatus: func(string) (runner.Status, error) {
			return runner.Status{State: runner.StateSuccess}, nil
		},
		hasSession: func(ctx context.Context, sessionName string) (bool, error) {
			return true, nil
		},
	})
	require.ErrorIs(t, err, ErrRunNotLive)
	require.ErrorContains(t, err, "run RUN123 is not live")
	require.ErrorContains(t, err, "evidence path: "+evidenceDir)
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

	root, evidenceDir := setupAttachRepo(t, "RUN123")
	_, err := resolveLiveRun(context.Background(), root, "RUN123", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			return run.IndexEntry{RunID: "RUN123", EvidencePath: filepath.Join(".tessariq", "runs", "RUN123")}, nil
		},
		readStatus: func(string) (runner.Status, error) {
			return runner.Status{State: runner.StateRunning}, nil
		},
		hasSession: func(ctx context.Context, sessionName string) (bool, error) {
			return false, nil
		},
	})
	require.ErrorIs(t, err, ErrRunNotLive)
	require.ErrorContains(t, err, "run RUN123 is not live")
	require.ErrorContains(t, err, "evidence path: "+evidenceDir)
	require.ErrorContains(t, err, "no live tmux session")
}

func TestResolveLiveRun_RejectsAbsoluteEvidencePathBeforeStatusRead(t *testing.T) {
	t.Parallel()

	root, _ := setupAttachRepo(t, "RUN123")
	_, err := resolveLiveRun(context.Background(), root, "RUN123", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			return run.IndexEntry{RunID: "RUN123", EvidencePath: "/tmp/evil-evidence"}, nil
		},
		readStatus: func(evidenceDir string) (runner.Status, error) {
			t.Fatalf("readStatus should not be called for invalid evidence path %q", evidenceDir)
			return runner.Status{}, nil
		},
		hasSession: func(ctx context.Context, sessionName string) (bool, error) {
			t.Fatalf("hasSession should not be called for invalid evidence path %q", sessionName)
			return false, nil
		},
	})
	require.ErrorIs(t, err, ErrRunNotLive)
	require.ErrorContains(t, err, "run RUN123 is not live")
	require.ErrorContains(t, err, "evidence path: /tmp/evil-evidence")
	require.ErrorContains(t, err, "outside the repository")
}

func TestResolveLiveRun_RejectsTraversalEvidencePathBeforeStatusRead(t *testing.T) {
	t.Parallel()

	root, _ := setupAttachRepo(t, "RUN123")
	_, err := resolveLiveRun(context.Background(), root, "RUN123", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			return run.IndexEntry{RunID: "RUN123", EvidencePath: filepath.Join(".tessariq", "runs", "..", "..", "outside")}, nil
		},
		readStatus: func(evidenceDir string) (runner.Status, error) {
			t.Fatalf("readStatus should not be called for invalid evidence path %q", evidenceDir)
			return runner.Status{}, nil
		},
		hasSession: func(ctx context.Context, sessionName string) (bool, error) {
			t.Fatalf("hasSession should not be called for invalid evidence path %q", sessionName)
			return false, nil
		},
	})
	require.ErrorIs(t, err, ErrRunNotLive)
	require.ErrorContains(t, err, "run RUN123 is not live")
	require.ErrorContains(t, err, filepath.Join(".tessariq", "runs", "..", "..", "outside"))
	require.ErrorContains(t, err, "outside the repository")
}

// TestResolveLiveRun_RejectsSymlinkedEvidenceOutsideRepo verifies that attach
// refuses a forged evidence directory that is a symlink whose real target
// escapes the repository, before any evidence file is read or any tmux
// session is touched.
func TestResolveLiveRun_RejectsSymlinkedEvidenceOutsideRepo(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	root, err := filepath.EvalSymlinks(rootDir)
	require.NoError(t, err)

	externalDir := t.TempDir()
	external, err := filepath.EvalSymlinks(externalDir)
	require.NoError(t, err)

	runID := "RUN_FORGED"
	forged := filepath.Join(external, runID)
	require.NoError(t, os.MkdirAll(forged, 0o755))

	runsDir := filepath.Join(root, ".tessariq", "runs")
	require.NoError(t, os.MkdirAll(runsDir, 0o755))
	require.NoError(t, os.Symlink(forged, filepath.Join(runsDir, runID)))

	_, err = resolveLiveRun(context.Background(), root, runID, dependencies{
		resolveRunRef: func(_, _ string) (run.IndexEntry, error) {
			return run.IndexEntry{RunID: runID, EvidencePath: filepath.Join(".tessariq", "runs", runID)}, nil
		},
		readStatus: func(string) (runner.Status, error) {
			t.Fatalf("readStatus must not be called for forged symlink evidence")
			return runner.Status{}, nil
		},
		hasSession: func(context.Context, string) (bool, error) {
			t.Fatalf("hasSession must not be called for forged symlink evidence")
			return false, nil
		},
	})
	require.ErrorIs(t, err, ErrRunNotLive)
	require.ErrorContains(t, err, "outside the repository")
}

func TestResolveLiveRun_RejectsMismatchedEvidenceRunIDBeforeStatusRead(t *testing.T) {
	t.Parallel()

	root, _ := setupAttachRepo(t, "RUN_B")
	_, err := resolveLiveRun(context.Background(), root, "RUN_A", dependencies{
		resolveRunRef: func(runsDir, ref string) (run.IndexEntry, error) {
			return run.IndexEntry{RunID: "RUN_A", EvidencePath: filepath.Join(".tessariq", "runs", "RUN_B")}, nil
		},
		readStatus: func(evidenceDir string) (runner.Status, error) {
			t.Fatalf("readStatus should not be called for mismatched evidence run ID %q", evidenceDir)
			return runner.Status{}, nil
		},
		hasSession: func(ctx context.Context, sessionName string) (bool, error) {
			t.Fatalf("hasSession should not be called for mismatched evidence run ID %q", sessionName)
			return false, nil
		},
	})
	require.ErrorIs(t, err, ErrRunNotLive)
	require.ErrorContains(t, err, "run RUN_A is not live")
	require.ErrorContains(t, err, "run_id mismatch")
}

func TestResolveLiveRun_SessionCheckErrorReturned(t *testing.T) {
	t.Parallel()

	root, _ := setupAttachRepo(t, "RUN123")
	_, err := resolveLiveRun(context.Background(), root, "RUN123", dependencies{
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
