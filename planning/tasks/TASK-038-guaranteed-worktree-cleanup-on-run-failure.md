---
id: TASK-038-guaranteed-worktree-cleanup-on-run-failure
title: Guarantee worktree cleanup on all post-provision run failure paths
status: done
priority: p0
depends_on:
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-028-container-session-streaming-and-cleanup-hardening
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#workspace-guarantees
    - specs/tessariq-v0.1.0.md#lifecycle-rules
updated_at: "2026-04-01T09:20:19Z"
areas:
    - workspace
    - cleanup
    - run
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Error-path cleanup guarantees must be pinned with deterministic tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Worktree and git cleanup behavior spans filesystem and git command execution.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Real run failures should not leak worktrees in user-visible workflows.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Cleanup defers and branching are frequent mutation weak points.
    manual_test:
        required: true
        commands: []
        rationale: Validate no leaked worktrees after intentionally failed runs.
---

## Summary

Worktree provisioning is not currently paired with unconditional cleanup across all downstream error paths, which can leak directories and stale git worktree entries. This task enforces guaranteed cleanup after provisioning, including failure scenarios.

## Supersedes

- BUG-006 from `planning/BUGS.md`.

## Acceptance Criteria

- After worktree provisioning succeeds, cleanup is guaranteed for all subsequent error returns in run setup/execution.
- Failed runs do not leave stale git worktree entries or leaked worktree directories.
- Successful runs preserve the expected workspace behavior and do not regress user-visible outputs.
- Cleanup remains safe and idempotent when workspace paths are already absent.

## Test Expectations

- Add unit tests for run command error branches verifying cleanup is invoked.
- Add integration tests that force failures after provisioning and assert no leaked worktree remains.
- Add e2e regression that intentionally fails run after provisioning and checks cleanup state.

## TDD Plan

1. RED: add failing test for an auth/config/process creation failure after provisioning asserting cleanup occurs.
2. RED: add failing test for runner execution failure asserting cleanup occurs.
3. GREEN: introduce scoped cleanup defer immediately after successful provisioning.
4. GREEN: ensure cleanup does not mask primary run errors.
5. REFACTOR: centralize cleanup/error composition if needed for readability.
6. GREEN: validate with integration/e2e cleanup assertions.

## Notes

- Likely files: `cmd/tessariq/run.go`, `internal/workspace/provision.go`, and related run/workspace tests.
- Keep cleanup path robust against partial initialization and teardown failures.
- 2026-04-01T09:20:19Z: Added defer-based worktree cleanup after successful Provision in run command. Cleanup is disarmed on success path, preserving worktree for attach/promote. Unit, integration, and e2e tests validate cleanup on failure and preservation on success. All manual test steps pass.
