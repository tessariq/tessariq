---
id: TASK-004-worktree-provisioning-and-workspace-metadata
title: Provision detached worktrees and record workspace metadata
status: todo
priority: p0
depends_on:
  - TASK-003-dirty-repo-gate-and-task-ingest
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
  - specs/tessariq-v0.1.0.md#workspace-guarantees
updated_at: 2026-03-29T00:00:00Z
areas:
  - git
  - workspace
verification:
  unit:
    required: true
    commands:
      - go test ./...
    rationale: Workspace-path derivation and metadata shaping should begin with unit tests.
  integration:
    required: true
    commands:
      - go test -tags=integration ./...
    rationale: Real worktree provisioning crosses process boundaries and the integration coverage must use Testcontainers-backed collaborators only.
  e2e:
    required: false
    commands:
      - go test -tags=e2e ./...
    rationale: Full CLI coverage can wait until run orchestration is complete.
  mutation:
    required: true
    commands:
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Provisioning and cleanup branching should be mutation-tested once implemented.
---

## Summary

Create detached worktrees under `~/.tessariq/worktrees/...` and emit `workspace.json`.

## Acceptance Criteria

- Worktrees are detached and isolated from the host working tree.
- `base_sha`, `workspace_path`, `repo_clean`, and reproducibility metadata are recorded.
- Cleanup paths are prepared for later runner and promote logic.

## Test Expectations

- Add unit tests for repo-id derivation and workspace metadata rendering.
- Add integration tests for worktree creation and cleanup, using Testcontainers-backed collaborators only.
- E2E tests are deferred until the full run command is operational.
- Run mutation testing because provisioning logic has multiple error paths.

## TDD Plan

- Start with a failing unit test for workspace metadata generation and a failing integration test for detached worktree creation.

## Notes

- Keep host path handling portable across Linux and macOS.
