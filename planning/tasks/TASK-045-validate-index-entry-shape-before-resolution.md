---
id: TASK-045-validate-index-entry-shape-before-resolution
title: Reject semantically invalid index lines before run-ref resolution
status: done
priority: p0
depends_on:
    - TASK-014-run-index-and-run-ref-resolution
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-02T07:40:49Z"
areas:
    - evidence
    - indexing
    - attach
    - promote
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Index-entry shape validation belongs in deterministic unit coverage first.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Corrupted-index behavior should be exercised through real file-backed index reads.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Invalid index entries affect user-visible attach and promote flows.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Validation and skip-or-reject branching is easy to regress silently.
    manual_test:
        required: true
        commands: []
        rationale: Confirms corrupted index lines are rejected cleanly in the CLI.
---

## Summary

`ReadIndex()` currently accepts any syntactically valid JSON object, even when required fields are missing. Harden index parsing so attach and promote only resolve semantically complete run entries.

## Supersedes

- BUG-021 from `planning/BUGS.md`.

## Acceptance Criteria

- Index entries missing any minimum required field (`run_id`, `created_at`, `task_path`, `task_title`, `agent`, `workspace_mode`, `state`, `evidence_path`) are rejected during index read or run-ref resolution.
- `last`, `last-N`, and explicit `run_id` resolution behave as though incomplete index lines do not exist.
- `attach last` and `promote last` do not probe the repository root or other zero-value-derived paths when the latest line is incomplete.
- Failure behavior remains clean and repo-scoped: if no valid run can be resolved, commands fail as no matching run found / empty index rather than acting on partial data.

## Test Expectations

- Add unit tests for `ReadIndex` and/or `ResolveRunRef` covering partial objects, empty strings in required fields, and valid minimum-shape entries.
- Add regression tests showing a single-line partial JSON object is ignored or rejected and does not resolve as `last`.
- Add integration or e2e coverage proving `attach last` and `promote last` fail cleanly against an index containing only incomplete entries.

## TDD Plan

1. RED: add a failing test for a partial JSON line with only `run_id`.
2. GREEN: validate required fields before accepting an index entry.
3. REFACTOR: keep malformed-vs-incomplete entry handling explicit and readable.
4. GREEN: verify attach/promote consumers do not act on rejected entries.

## Notes

- Likely files: `internal/run/index.go`, `internal/run/runref.go`, and attach/promote regression tests.
- Keep the append-only JSONL format; this task hardens reads and resolution, not the write contract.
- 2026-04-02T07:40:49Z: ReadIndex validates 8 required fields; incomplete entries silently skipped. Unit, integration, e2e, mutation, and manual tests pass.
