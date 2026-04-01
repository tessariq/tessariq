package attach

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/tmux"
)

var ErrRunNotLive = errors.New("run is not live")

type Result struct {
	RunID        string
	SessionName  string
	EvidencePath string
}

type dependencies struct {
	resolveRunRef func(runsDir, ref string) (run.IndexEntry, error)
	readStatus    func(evidenceDir string) (runner.Status, error)
	hasSession    func(ctx context.Context, name string) (bool, error)
}

func ResolveLiveRun(ctx context.Context, repoRoot, ref string) (Result, error) {
	return resolveLiveRun(ctx, repoRoot, ref, dependencies{
		resolveRunRef: run.ResolveRunRef,
		readStatus:    runner.ReadStatus,
		hasSession:    tmux.HasSession,
	})
}

func resolveLiveRun(ctx context.Context, repoRoot, ref string, deps dependencies) (Result, error) {
	runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
	entry, err := deps.resolveRunRef(runsDir, ref)
	if err != nil {
		if errors.Is(err, run.ErrRunNotFound) || errors.Is(err, run.ErrEmptyIndex) {
			return Result{}, fmt.Errorf("%w: run %s is not live: no matching run found", ErrRunNotLive, ref)
		}
		return Result{}, err
	}

	evidenceDir := entry.EvidencePath
	if !filepath.IsAbs(evidenceDir) {
		evidenceDir = filepath.Join(repoRoot, evidenceDir)
	}

	status, err := deps.readStatus(evidenceDir)
	if err != nil {
		return Result{}, fmt.Errorf("%w: run %s is not live; evidence path: %s: %v", ErrRunNotLive, entry.RunID, evidenceDir, err)
	}
	if status.State != runner.StateRunning {
		return Result{}, fmt.Errorf("%w: run %s is not live; state %s; evidence path: %s", ErrRunNotLive, entry.RunID, status.State, evidenceDir)
	}

	sessionName := run.SessionName(entry.RunID)
	exists, err := deps.hasSession(ctx, sessionName)
	if err != nil {
		return Result{}, fmt.Errorf("check tmux session for run %s: %w", entry.RunID, err)
	}
	if !exists {
		return Result{}, fmt.Errorf("%w: run %s is not live; no live tmux session; evidence path: %s", ErrRunNotLive, entry.RunID, evidenceDir)
	}

	return Result{
		RunID:        entry.RunID,
		SessionName:  sessionName,
		EvidencePath: evidenceDir,
	}, nil
}
