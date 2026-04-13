package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/workspace"
)

type dependencies struct {
	inspectContainer func(ctx context.Context, name string) (container.StateInfo, error)
	removeContainer  func(ctx context.Context, name string) error
	cleanupWorkspace func(ctx context.Context, repoRoot, workspacePath string) error
}

type Result struct {
	Entry  run.IndexEntry
	Status runner.Status
	Live   bool
}

// ReconcileRun normalizes stale running evidence into a terminal state so
// callers do not continue treating an orphaned run as live forever.
func ReconcileRun(ctx context.Context, repoRoot string, entry run.IndexEntry) (Result, error) {
	return reconcileRun(ctx, repoRoot, entry, dependencies{
		inspectContainer: container.InspectState,
		removeContainer:  container.Remove,
		cleanupWorkspace: workspace.Cleanup,
	})
}

func reconcileRun(ctx context.Context, repoRoot string, entry run.IndexEntry, deps dependencies) (Result, error) {
	evidenceDir, err := run.ValidateEvidencePath(repoRoot, entry.EvidencePath)
	if err != nil {
		return Result{}, err
	}
	if err := run.ValidateEvidenceRunID(evidenceDir, entry.RunID); err != nil {
		return Result{}, err
	}

	manifest, err := run.ReadManifest(evidenceDir)
	if err != nil {
		return Result{}, err
	}
	status, err := runner.ReadStatus(evidenceDir)
	if err != nil {
		return Result{}, err
	}

	updatedEntry := run.IndexEntryFromManifest(manifest, string(status.State))
	if status.State.IsTerminal() {
		if err := ensureIndexState(filepath.Join(repoRoot, ".tessariq", "runs"), manifest, string(status.State)); err != nil {
			return Result{}, err
		}
		if err := cleanupTerminalRun(ctx, repoRoot, evidenceDir, manifest, status.State, deps); err != nil {
			return Result{}, err
		}
		return Result{Entry: updatedEntry, Status: status, Live: false}, nil
	}
	if manifest.ContainerName == "" {
		return Result{Entry: entry, Status: status, Live: true}, nil
	}

	stateInfo, err := deps.inspectContainer(ctx, manifest.ContainerName)
	if err != nil {
		return Result{}, err
	}
	if stateInfo.Exists && stateInfo.Running {
		return Result{Entry: entry, Status: status, Live: true}, nil
	}
	if !stateInfo.Exists && !processStartObserved(evidenceDir) {
		return Result{Entry: entry, Status: status, Live: true}, nil
	}

	finalState, exitCode := inferReconciledState(timeoutFlagExists(evidenceDir), stateInfo.ExitCode)
	finishedAt := stateInfo.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}
	startedAt, err := parseStartedAt(status)
	if err != nil {
		return Result{}, err
	}

	status = runner.NewTerminalStatus(finalState, startedAt, finishedAt, exitCode, finalState == runner.StateTimeout)
	if err := runner.WriteStatus(evidenceDir, status); err != nil {
		return Result{}, err
	}
	if err := ensureIndexState(filepath.Join(repoRoot, ".tessariq", "runs"), manifest, string(finalState)); err != nil {
		return Result{}, err
	}
	if err := cleanupTerminalRun(ctx, repoRoot, evidenceDir, manifest, finalState, deps); err != nil {
		return Result{}, err
	}

	updatedEntry = run.IndexEntryFromManifest(manifest, string(finalState))
	return Result{Entry: updatedEntry, Status: status, Live: false}, nil
}

func ensureIndexState(runsDir string, manifest run.Manifest, state string) error {
	entries, err := run.ReadIndex(runsDir)
	if err != nil {
		return err
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].RunID != manifest.RunID {
			continue
		}
		if entries[i].State == state {
			return nil
		}
		break
	}
	return run.AppendIndex(runsDir, run.IndexEntryFromManifest(manifest, state))
}

func cleanupTerminalRun(ctx context.Context, repoRoot, evidenceDir string, manifest run.Manifest, state runner.State, deps dependencies) error {
	if deps.removeContainer != nil && manifest.ContainerName != "" {
		if err := deps.removeContainer(ctx, manifest.ContainerName); err != nil {
			return err
		}
	}
	if state == runner.StateSuccess || deps.cleanupWorkspace == nil {
		return nil
	}
	workspacePath, err := readWorkspacePath(evidenceDir)
	if err != nil {
		return err
	}
	if workspacePath == "" {
		return nil
	}
	return deps.cleanupWorkspace(ctx, repoRoot, workspacePath)
}

func inferReconciledState(timedOut bool, exitCode int) (runner.State, int) {
	if timedOut {
		return runner.StateTimeout, exitCode
	}
	switch exitCode {
	case 0:
		return runner.StateSuccess, 0
	case 130:
		return runner.StateInterrupted, exitCode
	case 137, 143:
		return runner.StateKilled, exitCode
	default:
		return runner.StateFailed, exitCode
	}
}

func timeoutFlagExists(evidenceDir string) bool {
	_, err := os.Stat(filepath.Join(evidenceDir, "timeout.flag"))
	return err == nil
}

func parseStartedAt(status runner.Status) (time.Time, error) {
	startedAt, err := time.Parse(time.RFC3339, status.StartedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse started_at: %w", err)
	}
	return startedAt, nil
}

func readWorkspacePath(evidenceDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(evidenceDir, "workspace.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read workspace metadata: %w", err)
	}
	var metadata workspace.Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", fmt.Errorf("parse workspace metadata: %w", err)
	}
	return metadata.WorkspacePath, nil
}

func processStartObserved(evidenceDir string) bool {
	data, err := os.ReadFile(filepath.Join(evidenceDir, "runner.log"))
	if err != nil {
		return false
	}
	text := string(data)
	return strings.Contains(text, "starting process") || strings.Contains(text, "starting interactive process")
}
