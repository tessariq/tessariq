---
id: TASK-046-unique-run-aware-last-run-resolution
title: Resolve last and last-N by unique runs instead of raw index lines
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#lifecycle-rules
dependencies:
    - TASK-014-run-index-and-run-ref-resolution
    - TASK-045-validate-index-entry-shape-before-resolution
updated_at: "2026-04-02T08:39:44Z"
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
- 2026-04-02T08:39:44Z: deduplicate index entries by run_id in resolveLastN; unit/integration/e2e/mutation tests pass
