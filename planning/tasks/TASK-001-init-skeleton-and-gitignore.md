---
id: TASK-001-init-skeleton-and-gitignore
title: Initialize repository skeleton and ignore generated runtime state
status: todo
priority: p0
depends_on: []
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#release-intent
    - specs/tessariq-v0.1.0.md#repository-model
    - specs/tessariq-v0.1.0.md#generated-runtime-state
    - specs/tessariq-v0.1.0.md#tessariq-init
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: "2026-03-29T12:06:20Z"
areas:
    - cli
    - repository
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Add focused unit tests for initialization helpers and ignore-file updates.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: No containerized collaborator is required if initialization remains local filesystem only.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: No end-to-end flow needs coverage until the CLI wiring exists.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Idempotent `.gitignore` insertion has branches (file exists vs not, entry present vs not, trailing newline handling) that mutations can weaken.
---

## Summary

Create `tessariq init` behavior for the repo skeleton and `.gitignore` update.

## Acceptance Criteria

- `specs/` and `.tessariq/runs/` are created when missing, with `.tessariq/` living at the repository root as a sibling of `specs/`.
- `.tessariq/` is added to `.gitignore` without duplicating entries, creating or updating `.gitignore` as needed.
- The command behaves cleanly on reruns.
- The task continues to treat `.tessariq/` as repo-local generated state, not repo-tracked config.

## Test Expectations

- Add or update unit tests for idempotent directory and ignore-file handling.
- Integration tests are not needed unless filesystem orchestration becomes multi-step enough to justify a containerized boundary.
- E2E tests are not needed yet because the broader CLI workflow is not exercised here.
- Run mutation testing because idempotent `.gitignore` handling has meaningful branch coverage.

## TDD Plan

- Start with a failing unit test for idempotent `.gitignore` insertion and required directory creation.

## Notes

- Keep the implementation behavior-preserving outside the new `init` command.
