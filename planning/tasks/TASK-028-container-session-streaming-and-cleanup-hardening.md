---
id: TASK-028-container-session-streaming-and-cleanup-hardening
title: Fix container session streaming and workspace cleanup hardening gaps
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#workspace-guarantees
dependencies:
    - TASK-027-container-lifecycle-and-mount-isolation
updated_at: "2026-03-31T16:37:29Z"
---

## Summary

Close the remaining TASK-027 gaps by streaming container output into the host tmux session via `run.log`, and by making disposable worktree cleanup reliable after non-host container writes.

## Acceptance Criteria

- The host tmux session for a run tails live output from `run.log` instead of starting empty.
- Container stdout and stderr are written durably to `run.log` for detached runs.
- `run.log` retains the full container output after the run finishes.
- Worktree cleanup succeeds even when the container created restrictive files or directories with a foreign UID/GID.
- Ownership/permission repair is limited to disposable worktree paths and does not broaden read-only evidence or auth/config mounts.
- Existing TASK-027 container lifecycle behavior remains intact.

## Test Expectations

- Add unit tests for runner output wiring and tmux session command construction.
- Add integration tests for container log capture into `run.log`.
- Add integration tests for cleanup after restrictive container-owned files are created in the worktree.
- Add a thin e2e test that verifies detached run output is visible through the host tmux session.
- Run mutation testing because the lifecycle and cleanup logic are safety-critical.

## TDD Plan

- Start with failing unit tests for session command construction and process output wiring, then add failing integration tests for run log capture and restrictive-worktree cleanup.

## Notes

- Keep tmux on the host side; the tmux session should observe container output through the durable log path rather than owning the container process directly.
- This task intentionally does not change proxy networking; TASK-012 still owns egress enforcement.
- 2026-03-31T16:37:29Z: Implemented detached run log streaming into host tmux sessions, durable container output capture, Docker-based disposable worktree ownership repair, and temporary --interactive fail-fast guidance. Evidence: go test ./...; go test -tags=integration ./...; go test -tags=e2e ./... -count=1; gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70; local-only manual-test and verification artifacts generated.
