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
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-03-29T12:06:20Z"
areas:
    - evidence
    - indexing
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Index entry shaping and `last`/`last-N` resolution belong in unit tests first.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Append-only file I/O with potential concurrent runs needs Testcontainers-backed validation for atomic append safety and corrupted-index resilience.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Run-ref resolution affects attach and promote user flows and deserves thin end-to-end coverage.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Index ordering and run-ref parsing are branch-heavy.
    manual_test:
        required: true
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
---

## Summary

Append run index entries and implement repository-scoped run-ref resolution.

## Acceptance Criteria

- `index.jsonl` is append-only.
- `index.jsonl` entries include the minimum required fields for `run_id`, `created_at`, `task_path`, `task_title`, `adapter`, `workspace_mode`, `state`, and `evidence_path`.
- `run_id`, `last`, and `last-N` resolve against the current repository's run index only.
- Commands fail cleanly when the referenced run cannot be found in the current repository.
- Shared run-ref resolution is the source of truth for attach and promote.

## Test Expectations

- Add unit tests for index entry rendering and run-ref resolution.
- Add unit tests for edge cases: `last` on empty index, `last-0` behavior, malformed JSON lines in index (graceful skip or explicit failure), and ULID format validation of index entries.
- Add integration tests for concurrent append safety (two runs finishing simultaneously) and corrupted-index recovery.
- Add thin e2e coverage because run refs feed user-facing attach and promote commands.
- Run mutation testing because ordering and resolution logic are subtle.

## TDD Plan

- Start with a failing unit test for `last-N` resolution and append-only index behavior.

## Notes

- Do not rewrite index entries in place for another run.
