---
id: TASK-046-unique-run-aware-last-run-resolution
title: Resolve last and last-N by unique runs instead of raw index lines
status: todo
priority: p1
depends_on:
    - TASK-014-run-index-and-run-ref-resolution
    - TASK-045-validate-index-entry-shape-before-resolution
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-04-01T20:03:47Z"
areas:
    - indexing
    - attach
    - promote
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Unique-run resolution semantics should be proven in focused unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Index files with repeated lifecycle entries need realistic file-backed coverage.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: `last-N` directly affects user-facing attach and promote commands.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Off-by-one and de-duplication logic are mutation-prone.
    manual_test:
        required: true
        commands: []
        rationale: Confirms `last-1` selects the previous run rather than an earlier lifecycle entry for the same run.
---

## Summary

Run-ref resolution currently counts raw `index.jsonl` lines, but Tessariq appends multiple lifecycle entries for a single run. Make `last` and `last-N` operate on unique `run_id` values so previous-run selection matches the spec's run-scoped semantics.

## Supersedes

- BUG-020 from `planning/BUGS.md`.

## Acceptance Criteria

- `last` resolves to the newest unique run, not merely the last JSONL line.
- `last-1` resolves to the previous unique run even when the newest run has multiple appended index entries.
- Explicit `run_id` resolution remains unchanged and still returns the latest entry for that run.
- Resolution order remains append-order based across unique runs.

## Test Expectations

- Add unit tests for indexes containing repeated `running` and terminal entries for the same `run_id`.
- Add regression coverage for `last`, `last-0`, and `last-1` when the latest run appears more than once.
- Add integration or e2e coverage showing `promote last-1` selects the earlier run in a duplicate-entry index.

## TDD Plan

1. RED: add a failing test with `RUN_A`, `RUN_B running`, `RUN_B success`.
2. GREEN: de-duplicate by `run_id` while preserving latest-first run ordering.
3. REFACTOR: keep explicit `run_id` lookup semantics separate from `last-N` semantics.
4. GREEN: verify attach/promote still resolve the latest lifecycle entry for explicit run IDs.

## Notes

- Likely files: `internal/run/runref.go` and associated tests.
- Keep append-only index writes intact; this task changes resolution semantics, not index emission.
