package adapter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/tessariq/tessariq/internal/authmount"
)

// RuntimeState is the result of materializing the disposable per-run
// runtime-state layer for one run. EffectiveMounts is the transformed
// MountSpec list where every spec with SeedIntoRuntime=true has been
// rewritten to point at a disposable scratch file (read-write). Cleanup
// removes the scratch tree and is safe to call multiple times.
type RuntimeState struct {
	EffectiveMounts []authmount.MountSpec

	cleanupOnce sync.Once
	cleanupErr  error
	scratchRoot string
}

// Cleanup removes the per-run scratch directory. It is idempotent.
func (r *RuntimeState) Cleanup() error {
	r.cleanupOnce.Do(func() {
		if r.scratchRoot == "" {
			return
		}
		r.cleanupErr = os.RemoveAll(r.scratchRoot)
	})
	return r.cleanupErr
}

// PrepareRuntimeState materializes disposable per-run copies for every
// MountSpec with SeedIntoRuntime=true. For each seed spec it:
//
//  1. copies the host source into <scratchRoot>/<basename> at mode 0o600, and
//  2. replaces the returned spec's HostPath with the scratch path,
//     clears SeedIntoRuntime, and sets ReadOnly=false so the caller binds
//     the scratch file read-write into the container at the original
//     ContainerPath.
//
// Non-seed specs pass through unchanged. If any seed copy fails, the
// scratch root is removed before the error is returned so there is no
// partial state to leak.
//
// The caller owns scratchRoot and must choose a per-run path that is not
// shared across runs and not mounted anywhere else in the container; see
// cmd/tessariq/run.go for the `~/.tessariq/runtime-state/<run_id>/` layout.
func PrepareRuntimeState(scratchRoot string, specs []authmount.MountSpec) (*RuntimeState, error) {
	rs := &RuntimeState{scratchRoot: scratchRoot}

	if !hasSeedSpec(specs) {
		rs.EffectiveMounts = append([]authmount.MountSpec(nil), specs...)
		return rs, nil
	}

	if err := os.MkdirAll(scratchRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create runtime-state scratch root: %w", err)
	}

	out := make([]authmount.MountSpec, 0, len(specs))
	for _, s := range specs {
		if !s.SeedIntoRuntime {
			out = append(out, s)
			continue
		}

		scratchPath := filepath.Join(scratchRoot, filepath.Base(s.ContainerPath))
		if err := copyFile(s.HostPath, scratchPath); err != nil {
			_ = os.RemoveAll(scratchRoot)
			return nil, fmt.Errorf("seed runtime-state for %s: %w", s.ContainerPath, err)
		}

		out = append(out, authmount.MountSpec{
			HostPath:        scratchPath,
			ContainerPath:   s.ContainerPath,
			ReadOnly:        false,
			SeedIntoRuntime: false,
		})
	}

	rs.EffectiveMounts = out
	return rs, nil
}

func hasSeedSpec(specs []authmount.MountSpec) bool {
	for _, s := range specs {
		if s.SeedIntoRuntime {
			return true
		}
	}
	return false
}

// copyFile copies src to dst, creating dst with mode 0o600. It fails if
// src cannot be read, or if dst already exists with conflicting content.
// The destination is written with O_EXCL to make it obvious that the
// scratch tree was not reused across runs.
func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304 -- src is a discovered auth path supplied by the caller
	if err != nil {
		return fmt.Errorf("open host source: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create scratch file: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("copy host source to scratch: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("close scratch file: %w", err)
	}
	return nil
}
