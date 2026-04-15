package lifecycle

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/workspace"
)

func TestReconcileRun_ExitedContainerWritesTerminalStatusAndIndex(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
	wsPath := workspace.WorkspacePath(homeDir, repoRoot, runID)

	require.NoError(t, os.MkdirAll(evidenceDir, 0o700))
	require.NoError(t, os.MkdirAll(wsPath, 0o755))

	manifest := run.Manifest{
		SchemaVersion:       1,
		RunID:               runID,
		TaskPath:            "tasks/sample.md",
		TaskTitle:           "Sample",
		Agent:               "claude-code",
		WorkspaceMode:       "worktree",
		ContainerName:       run.ContainerName(runID),
		CreatedAt:           "2026-01-01T00:00:00Z",
		RequestedEgressMode: "open",
		ResolvedEgressMode:  "open",
		AllowlistSource:     "cli",
	}
	require.NoError(t, run.WriteManifest(evidenceDir, manifest))
	require.NoError(t, runner.WriteStatus(evidenceDir, runner.NewInitialStatus(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))))
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata("abc123", wsPath)))

	entry := run.IndexEntryFromManifest(manifest, string(runner.StateRunning))
	require.NoError(t, run.AppendIndex(runsDir, entry))

	result, err := reconcileRun(context.Background(), repoRoot, entry, dependencies{
		homeDir: homeDir,
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			require.Equal(t, run.ContainerName(runID), name)
			return container.StateInfo{
				Exists:     true,
				Running:    false,
				ExitCode:   0,
				FinishedAt: time.Date(2026, 1, 1, 0, 2, 0, 0, time.UTC),
			}, nil
		},
		removeContainer: func(ctx context.Context, name string) error {
			require.Equal(t, run.ContainerName(runID), name)
			return nil
		},
		cleanupWorkspace: func(ctx context.Context, homeDir, repoRoot, workspacePath string) error {
			t.Fatalf("successful reconciled runs must not clean the worktree")
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, runner.StateSuccess, result.Status.State)
	require.Equal(t, "success", result.Entry.State)
	require.False(t, result.Live)

	status, err := runner.ReadStatus(evidenceDir)
	require.NoError(t, err)
	require.Equal(t, runner.StateSuccess, status.State)
	require.Equal(t, 0, status.ExitCode)
	require.Equal(t, "2026-01-01T00:02:00Z", status.FinishedAt)

	entries, err := run.ReadIndex(runsDir)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "running", entries[0].State)
	require.Equal(t, "success", entries[1].State)
}

func TestReconcileRun_TerminalNonSuccessCleansWorkspace(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	wsPath := workspace.WorkspacePath(homeDir, repoRoot, runID)

	require.NoError(t, os.MkdirAll(evidenceDir, 0o700))
	require.NoError(t, os.MkdirAll(wsPath, 0o755))

	manifest := run.Manifest{
		SchemaVersion:       1,
		RunID:               runID,
		TaskPath:            "tasks/sample.md",
		TaskTitle:           "Sample",
		Agent:               "claude-code",
		WorkspaceMode:       "worktree",
		ContainerName:       run.ContainerName(runID),
		CreatedAt:           "2026-01-01T00:00:00Z",
		RequestedEgressMode: "open",
		ResolvedEgressMode:  "open",
		AllowlistSource:     "cli",
	}
	require.NoError(t, run.WriteManifest(evidenceDir, manifest))
	require.NoError(t, runner.WriteStatus(evidenceDir, runner.NewTerminalStatus(
		runner.StateFailed,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		1,
		false,
	)))
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata("abc123", wsPath)))

	entry := run.IndexEntryFromManifest(manifest, string(runner.StateRunning))

	cleaned := false
	result, err := reconcileRun(context.Background(), repoRoot, entry, dependencies{
		homeDir: homeDir,
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			return container.StateInfo{}, nil
		},
		cleanupWorkspace: func(ctx context.Context, gotHomeDir, gotRepoRoot, workspacePath string) error {
			cleaned = true
			require.Equal(t, homeDir, gotHomeDir)
			require.Equal(t, wsPath, workspacePath)
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, runner.StateFailed, result.Status.State)
	require.True(t, cleaned)
}

// TestReconcileRun_IdempotentWhenWorkspaceAlreadyRemoved pins the BUG-060
// fix: a terminal non-success run whose canonical worktree has already
// been removed (prior reconcile, manual cleanup, idempotent re-entry)
// must still be reconcilable. ValidateWorkspacePath short-circuits the
// ENOENT case, and the cleanupWorkspace hook is invoked with the
// canonical path so Cleanup's own os.Stat ENOENT fast path can no-op.
func TestReconcileRun_IdempotentWhenWorkspaceAlreadyRemoved(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FB1"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	wsPath := workspace.WorkspacePath(homeDir, repoRoot, runID)

	require.NoError(t, os.MkdirAll(evidenceDir, 0o700))
	// Intentionally do NOT create wsPath — the worktree has already been
	// removed by a prior reconcile or manual cleanup.

	manifest := run.Manifest{
		SchemaVersion:       1,
		RunID:               runID,
		TaskPath:            "tasks/sample.md",
		TaskTitle:           "Sample",
		Agent:               "claude-code",
		WorkspaceMode:       "worktree",
		ContainerName:       run.ContainerName(runID),
		CreatedAt:           "2026-01-01T00:00:00Z",
		RequestedEgressMode: "open",
		ResolvedEgressMode:  "open",
		AllowlistSource:     "cli",
	}
	require.NoError(t, run.WriteManifest(evidenceDir, manifest))
	require.NoError(t, runner.WriteStatus(evidenceDir, runner.NewTerminalStatus(
		runner.StateFailed,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		1,
		false,
	)))
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata("abc123", wsPath)))

	entry := run.IndexEntryFromManifest(manifest, string(runner.StateRunning))

	cleanedCalls := 0
	result, err := reconcileRun(context.Background(), repoRoot, entry, dependencies{
		homeDir: homeDir,
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			return container.StateInfo{}, nil
		},
		removeContainer: func(ctx context.Context, name string) error { return nil },
		cleanupWorkspace: func(ctx context.Context, gotHomeDir, gotRepoRoot, workspacePath string) error {
			cleanedCalls++
			require.Equal(t, homeDir, gotHomeDir)
			require.Equal(t, repoRoot, gotRepoRoot)
			require.Equal(t, wsPath, workspacePath)
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, runner.StateFailed, result.Status.State)
	require.False(t, result.Live)
	require.Equal(t, 1, cleanedCalls, "cleanupWorkspace must still be invoked exactly once so Cleanup's own ENOENT fast path can run")
}

func TestReconcileRun_MissingContainerAfterStartTreatsAsFailed(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
	wsPath := workspace.WorkspacePath(homeDir, repoRoot, runID)

	require.NoError(t, os.MkdirAll(evidenceDir, 0o700))
	require.NoError(t, os.MkdirAll(wsPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "runner.log"), []byte("[2026-01-01T00:00:00Z] starting process\n"), 0o600))

	manifest := run.Manifest{
		SchemaVersion:       1,
		RunID:               runID,
		TaskPath:            "tasks/sample.md",
		TaskTitle:           "Sample",
		Agent:               "claude-code",
		WorkspaceMode:       "worktree",
		ContainerName:       run.ContainerName(runID),
		CreatedAt:           "2026-01-01T00:00:00Z",
		RequestedEgressMode: "open",
		ResolvedEgressMode:  "open",
		AllowlistSource:     "cli",
	}
	require.NoError(t, run.WriteManifest(evidenceDir, manifest))
	require.NoError(t, runner.WriteStatus(evidenceDir, runner.NewInitialStatus(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))))
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata("abc123", wsPath)))

	entry := run.IndexEntryFromManifest(manifest, string(runner.StateRunning))
	require.NoError(t, run.AppendIndex(runsDir, entry))

	cleaned := false
	result, err := reconcileRun(context.Background(), repoRoot, entry, dependencies{
		homeDir: homeDir,
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			require.Equal(t, run.ContainerName(runID), name)
			return container.StateInfo{Exists: false}, nil
		},
		removeContainer: func(ctx context.Context, name string) error {
			return nil
		},
		cleanupWorkspace: func(ctx context.Context, gotHomeDir, gotRepoRoot, workspacePath string) error {
			cleaned = true
			require.Equal(t, homeDir, gotHomeDir)
			require.Equal(t, wsPath, workspacePath)
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, runner.StateFailed, result.Status.State)
	require.Equal(t, -1, result.Status.ExitCode)
	require.Equal(t, "failed", result.Entry.State)
	require.False(t, result.Live)
	require.True(t, cleaned, "failed reconciled runs must clean the worktree")

	status, err := runner.ReadStatus(evidenceDir)
	require.NoError(t, err)
	require.Equal(t, runner.StateFailed, status.State)
	require.Equal(t, -1, status.ExitCode)

	entries, err := run.ReadIndex(runsDir)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "running", entries[0].State)
	require.Equal(t, "failed", entries[1].State)
}

func TestReconcileRun_MissingContainerWithTimeoutFlag(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAY"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	wsPath := workspace.WorkspacePath(homeDir, repoRoot, runID)

	require.NoError(t, os.MkdirAll(evidenceDir, 0o700))
	require.NoError(t, os.MkdirAll(wsPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "runner.log"), []byte("[2026-01-01T00:00:00Z] starting process\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(evidenceDir, "timeout.flag"), []byte{}, 0o600))

	manifest := run.Manifest{
		SchemaVersion:       1,
		RunID:               runID,
		TaskPath:            "tasks/sample.md",
		TaskTitle:           "Sample",
		Agent:               "claude-code",
		WorkspaceMode:       "worktree",
		ContainerName:       run.ContainerName(runID),
		CreatedAt:           "2026-01-01T00:00:00Z",
		RequestedEgressMode: "open",
		ResolvedEgressMode:  "open",
		AllowlistSource:     "cli",
	}
	require.NoError(t, run.WriteManifest(evidenceDir, manifest))
	require.NoError(t, runner.WriteStatus(evidenceDir, runner.NewInitialStatus(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))))
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata("abc123", wsPath)))

	entry := run.IndexEntryFromManifest(manifest, string(runner.StateRunning))

	result, err := reconcileRun(context.Background(), repoRoot, entry, dependencies{
		homeDir: homeDir,
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			return container.StateInfo{Exists: false}, nil
		},
		removeContainer:  func(ctx context.Context, name string) error { return nil },
		cleanupWorkspace: func(ctx context.Context, homeDir, repoRoot, workspacePath string) error { return nil },
	})
	require.NoError(t, err)
	require.Equal(t, runner.StateTimeout, result.Status.State)
	require.Equal(t, -1, result.Status.ExitCode)
	require.Equal(t, "timeout", result.Entry.State)
}

// TestReconcileRun_CreatedContainerTreatedAsLive verifies that a container in
// the "created" state (docker create completed, docker start not yet) is
// reported as Live rather than inferred as a successful terminal run. A
// just-created container reports Running=false, ExitCode=0, and a zero-valued
// FinishedAt, which is indistinguishable from "exited cleanly" by shape alone
// — reconcile must use the zero FinishedAt to tell them apart.
func TestReconcileRun_CreatedContainerTreatedAsLive(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAZ"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	wsPath := workspace.WorkspacePath(homeDir, repoRoot, runID)

	require.NoError(t, os.MkdirAll(evidenceDir, 0o700))
	require.NoError(t, os.MkdirAll(wsPath, 0o755))

	manifest := run.Manifest{
		SchemaVersion:       1,
		RunID:               runID,
		TaskPath:            "tasks/sample.md",
		TaskTitle:           "Sample",
		Agent:               "claude-code",
		WorkspaceMode:       "worktree",
		ContainerName:       run.ContainerName(runID),
		CreatedAt:           "2026-01-01T00:00:00Z",
		RequestedEgressMode: "open",
		ResolvedEgressMode:  "open",
		AllowlistSource:     "cli",
	}
	require.NoError(t, run.WriteManifest(evidenceDir, manifest))
	require.NoError(t, runner.WriteStatus(evidenceDir, runner.NewInitialStatus(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))))
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata("abc123", wsPath)))

	entry := run.IndexEntryFromManifest(manifest, string(runner.StateRunning))

	result, err := reconcileRun(context.Background(), repoRoot, entry, dependencies{
		homeDir: homeDir,
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			require.Equal(t, run.ContainerName(runID), name)
			return container.StateInfo{
				Exists:     true,
				Running:    false,
				ExitCode:   0,
				FinishedAt: time.Time{},
			}, nil
		},
		removeContainer: func(ctx context.Context, name string) error {
			t.Fatalf("a created container must not be removed by reconcile — the runner is mid-start")
			return nil
		},
		cleanupWorkspace: func(ctx context.Context, homeDir, repoRoot, workspacePath string) error {
			t.Fatalf("a created container must not trigger workspace cleanup")
			return nil
		},
	})
	require.NoError(t, err)
	require.True(t, result.Live, "created-but-not-yet-started container must be reported as live")
	require.Equal(t, runner.StateRunning, result.Status.State, "status must not be rewritten while container is still in created state")

	status, err := runner.ReadStatus(evidenceDir)
	require.NoError(t, err)
	require.Equal(t, runner.StateRunning, status.State, "status.json must not be overwritten with a bogus terminal state")
}

func TestInferReconciledState_InterruptedExitCode(t *testing.T) {
	t.Parallel()

	state, exitCode := inferReconciledState(false, 130)
	require.Equal(t, runner.StateInterrupted, state)
	require.Equal(t, 130, exitCode)
}

// TestReconcileRun_RejectsTamperedWorkspacePath guards against the BUG-055
// arbitrary-path chown/chmod/delete primitive. A non-success run whose
// workspace.json points at a decoy outside <homeDir>/.tessariq/worktrees/
// must fail reconcile without invoking the cleanup pipeline and without
// touching the decoy.
func TestReconcileRun_RejectsTamperedWorkspacePath(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FB0"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	require.NoError(t, os.MkdirAll(evidenceDir, 0o700))

	manifest := run.Manifest{
		SchemaVersion:       1,
		RunID:               runID,
		TaskPath:            "tasks/sample.md",
		TaskTitle:           "Sample",
		Agent:               "claude-code",
		WorkspaceMode:       "worktree",
		ContainerName:       run.ContainerName(runID),
		CreatedAt:           "2026-01-01T00:00:00Z",
		RequestedEgressMode: "open",
		ResolvedEgressMode:  "open",
		AllowlistSource:     "cli",
	}
	require.NoError(t, run.WriteManifest(evidenceDir, manifest))
	require.NoError(t, runner.WriteStatus(evidenceDir, runner.NewTerminalStatus(
		runner.StateFailed,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		1,
		false,
	)))

	// Plant a tampered workspace.json whose workspace_path points at a
	// decoy outside ~/.tessariq/worktrees/ entirely.
	decoyDir := filepath.Join(t.TempDir(), "decoy")
	decoySentinel := filepath.Join(decoyDir, "sentinel.txt")
	require.NoError(t, os.MkdirAll(decoyDir, 0o755))
	require.NoError(t, os.WriteFile(decoySentinel, []byte("do-not-touch"), 0o600))
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata("abc123", decoyDir)))

	entry := run.IndexEntryFromManifest(manifest, string(runner.StateRunning))

	_, err := reconcileRun(context.Background(), repoRoot, entry, dependencies{
		homeDir: homeDir,
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			return container.StateInfo{}, nil
		},
		removeContainer: func(ctx context.Context, name string) error { return nil },
		cleanupWorkspace: func(ctx context.Context, homeDir, repoRoot, workspacePath string) error {
			t.Fatalf("cleanup must not be invoked when workspace_path is tampered; got %s", workspacePath)
			return nil
		},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, workspace.ErrWorkspacePathOutsideTree)

	// Decoy must be untouched.
	info, statErr := os.Stat(decoyDir)
	require.NoError(t, statErr)
	require.True(t, info.IsDir())
	data, readErr := os.ReadFile(decoySentinel)
	require.NoError(t, readErr)
	require.Equal(t, "do-not-touch", string(data))
}

func TestReadWorkspacePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, workspace.WriteMetadata(dir, workspace.BuildMetadata("abc123", "/tmp/worktree")))

	path, err := readWorkspacePath(dir)
	require.NoError(t, err)
	require.Equal(t, "/tmp/worktree", path)

	data, err := os.ReadFile(filepath.Join(dir, "workspace.json"))
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Contains(t, raw, "workspace_path")
}
