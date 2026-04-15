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
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
	wsPath := filepath.Join(repoRoot, "worktree")

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
		cleanupWorkspace: func(ctx context.Context, repoRoot, workspacePath string) error {
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
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	wsPath := filepath.Join(repoRoot, "worktree")

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
	require.NoError(t, workspace.WriteMetadata(evidenceDir, workspace.BuildMetadata("abc123", wsPath)))

	entry := run.IndexEntryFromManifest(manifest, string(runner.StateRunning))

	cleaned := false
	result, err := reconcileRun(context.Background(), repoRoot, entry, dependencies{
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			return container.StateInfo{}, nil
		},
		cleanupWorkspace: func(ctx context.Context, repoRoot, workspacePath string) error {
			cleaned = true
			require.Equal(t, wsPath, workspacePath)
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, runner.StateFailed, result.Status.State)
	require.True(t, cleaned)
}

func TestReconcileRun_MissingContainerAfterStartTreatsAsFailed(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
	wsPath := filepath.Join(repoRoot, "worktree")

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
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			require.Equal(t, run.ContainerName(runID), name)
			return container.StateInfo{Exists: false}, nil
		},
		removeContainer: func(ctx context.Context, name string) error {
			return nil
		},
		cleanupWorkspace: func(ctx context.Context, repoRoot, workspacePath string) error {
			cleaned = true
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
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAY"
	evidenceDir := filepath.Join(repoRoot, ".tessariq", "runs", runID)
	wsPath := filepath.Join(repoRoot, "worktree")

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
		inspectContainer: func(ctx context.Context, name string) (container.StateInfo, error) {
			return container.StateInfo{Exists: false}, nil
		},
		removeContainer:  func(ctx context.Context, name string) error { return nil },
		cleanupWorkspace: func(ctx context.Context, repoRoot, workspacePath string) error { return nil },
	})
	require.NoError(t, err)
	require.Equal(t, runner.StateTimeout, result.Status.State)
	require.Equal(t, -1, result.Status.ExitCode)
	require.Equal(t, "timeout", result.Entry.State)
}

func TestInferReconciledState_InterruptedExitCode(t *testing.T) {
	t.Parallel()

	state, exitCode := inferReconciledState(false, 130)
	require.Equal(t, runner.StateInterrupted, state)
	require.Equal(t, 130, exitCode)
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
