---
id: TASK-014-run-index-and-run-ref-resolution
title: Maintain the run index and resolve repository-scoped run refs
status: todo
priority: p1
depends_on:
  - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
  - TASK-013-diff-log-and-evidence-artifacts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
  - specs/tessariq-v0.1.0.md#lifecycle-rules
updated_at: 2026-03-29T00:00:00Z
areas:
  - evidence
  - indexing
verification:
  unit:
    required: true
    commands:
      - go test ./...
    rationale: "Index entry shaping and `last`/`last-N` resolution belong in unit tests first."
  integration:
    required: false
    commands:
      - go test -tags=integration ./...
    rationale: Containerized integration coverage is only needed if file append behavior requires process-level validation.
  e2e:
    required: true
    commands:
      - go test -tags=e2e ./...
    rationale: Run-ref resolution affects attach and promote user flows and deserves thin end-to-end coverage.
  mutation:
    required: true
    commands:
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Index ordering and run-ref parsing are branch-heavy.
---

## Summary

Append run index entries and implement repository-scoped run-ref resolution.

## Acceptance Criteria

- `index.jsonl` is append-only.
- `run_id`, `last`, and `last-N` resolve against the current repository.
- Unknown refs fail cleanly.

## Test Expectations

- Add unit tests for index entry rendering and run-ref resolution.
- Integration tests are optional unless append semantics need process-level validation.
- Add thin e2e coverage because run refs feed user-facing attach and promote commands.
- Run mutation testing because ordering and resolution logic are subtle.

## TDD Plan

- Start with a failing unit test for `last-N` resolution and append-only index behavior.

## Notes

- Do not rewrite index entries in place for another run.
