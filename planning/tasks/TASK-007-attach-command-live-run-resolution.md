---
id: TASK-007-attach-command-live-run-resolution
title: Implement attach command and live run resolution
status: todo
priority: p1
depends_on:
  - TASK-006-tmux-session-and-detached-attach-guidance
  - TASK-014-run-index-and-run-ref-resolution
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
  - specs/tessariq-v0.1.0.md#tessariq-attach-run-ref
  - specs/tessariq-v0.1.0.md#lifecycle-rules
  - specs/tessariq-v0.1.0.md#acceptance-scenarios
  - specs/tessariq-v0.1.0.md#failure-ux
updated_at: 2026-03-29T00:00:00Z
areas:
  - tmux
  - cli
verification:
  unit:
    required: true
    commands:
      - go test ./...
    rationale: Run-ref parsing and live-run eligibility checks should start with unit tests.
  integration:
    required: true
    commands:
      - go test -tags=integration ./...
    rationale: Attach behavior relies on real session/process resolution and must use Testcontainers-backed integration coverage only.
  e2e:
    required: true
    commands:
      - go test -tags=e2e ./...
    rationale: A thin attach end-to-end flow is needed because the feature is explicitly user-visible.
  mutation:
    required: true
    commands:
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Run-ref and eligibility branching should survive mutation testing.
---

## Summary

Implement `tessariq attach <run-ref>` on top of the shared repository-scoped run-ref resolver.

## Acceptance Criteria

- Attach works only for live runs.
- Unknown or finished runs fail cleanly.
- Failure output includes the evidence path when it is known.

## Test Expectations

- Add unit tests for attach preflight decisions and live-run eligibility on top of the shared run-ref resolver.
- Add integration tests for live-session lookup and attach failures using Testcontainers-backed collaborators only.
- Add a thin e2e attach flow for a live run.
- Run mutation testing because the resolution logic has multiple branches.

## TDD Plan

- Start with a failing unit test for attach live-run eligibility, then a failing e2e test for attaching to a live run.

## Notes

- Shared run-ref parsing and index semantics are intentionally owned by `TASK-014`.
