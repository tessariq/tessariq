---
id: TASK-061-cleanup-worktrees-even-when-ownership-repair-fails
title: Continue worktree cleanup when ownership repair fails
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#workspace-guarantees
dependencies:
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-031-pin-repair-container-image
    - TASK-038-guaranteed-worktree-cleanup-on-run-failure
updated_at: "2026-04-03T08:56:28Z"
---

## Summary

`workspace.Cleanup` returns immediately when ownership repair fails, which can strand both the filesystem path and the git worktree ref. Cleanup should keep attempting `git worktree remove` and local removal even after the repair step fails.

## Supersedes

- BUG-028 from `planning/BUGS.md`.

## Acceptance Criteria

- Cleanup still attempts `git.RemoveWorktree` and `os.RemoveAll` after a repair-container failure.
- A failed repair step is surfaced as context, but it does not prevent best-effort worktree teardown.
- Repeated cleanup remains idempotent when the worktree path or ref is already gone.
- Post-failure warnings identify any residual leak accurately if teardown still cannot complete.

## Test Expectations

- Add unit tests for the control flow where repair fails but later cleanup steps still run.
- Add integration coverage simulating a repair failure and asserting the git worktree entry is removed.
- Add regression coverage for the normal successful cleanup path.

## TDD Plan

1. RED: add a failing cleanup test where ownership repair errors out.
2. GREEN: continue to worktree removal and local deletion despite the repair error.
3. REFACTOR: aggregate cleanup errors without short-circuiting the rest of the teardown.

## Notes

- Likely files: `internal/workspace/provision.go` and workspace cleanup tests.
- A host-side `chmod -R u+rwX` fallback may be enough when the repair container is unavailable.
- 2026-04-03T08:56:28Z: Cleanup no longer short-circuits on repair failure; continues with host chmod fallback, git worktree remove, and os.RemoveAll. Unit + integration + mutation + manual tests all pass.
