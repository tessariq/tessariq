---
id: TASK-061-cleanup-worktrees-even-when-ownership-repair-fails
title: Continue worktree cleanup when ownership repair fails
status: todo
priority: p1
depends_on:
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-031-pin-repair-container-image
    - TASK-038-guaranteed-worktree-cleanup-on-run-failure
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#workspace-guarantees
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-02T14:59:17Z"
areas:
    - workspace
    - cleanup
    - reliability
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Cleanup fallback control flow should be verified deterministically before any Docker-backed checks.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Real git-worktree removal behavior must be exercised when repair fails.
    e2e:
        required: false
        commands: []
        rationale: Integration coverage should be enough for the narrow cleanup path.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Failure-path cleanup logic is branch-heavy and easy to regress.
    manual_test:
        required: true
        commands: []
        rationale: Confirms leaked worktrees do not remain when Docker becomes unavailable mid-run.
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
