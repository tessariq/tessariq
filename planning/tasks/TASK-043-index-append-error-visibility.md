---
id: TASK-043-index-append-error-visibility
title: Make run index append failures visible to users
status: done
priority: p0
depends_on:
    - TASK-014-run-index-and-run-ref-resolution
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#tessariq-attach-run-ref
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-01T18:06:11Z"
areas:
    - indexing
    - cli
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Index append and warning behavior should be covered with deterministic stubs.
    integration:
        required: false
        commands: []
        rationale: Behavior is local file/evidence error handling and can be covered in unit tests.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Validate user-visible warning and run-ref behavior under partial evidence failures.
    mutation:
        required: false
        commands: []
        rationale: This is localized warning-path hardening.
    manual_test:
        required: true
        commands: []
        rationale: Confirm CLI warns while preserving run completion semantics.
---

## Summary

`appendIndexEntry` currently drops read/append errors silently, which can leave successful runs missing from `.tessariq/runs/index.jsonl` without any user signal.

## Supersedes

- BUG-011 from `planning/BUGS.md`.

## Acceptance Criteria

- Failures reading manifest/status for index construction emit a warning to stderr with actionable context.
- Failures appending to `index.jsonl` emit a warning to stderr.
- Primary run result/evidence behavior remains unchanged (index stays supplementary), but missing index writes are never silent.
- Warning text is stable and test-covered.

## Test Expectations

- Add unit tests for warning emission on each failure branch in `appendIndexEntry`.
- Add unit test ensuring successful append path does not emit warnings.
- Add e2e regression where index append is forced to fail (e.g., permissions) and warning is visible.

## TDD Plan

1. RED: add failing tests asserting warnings for manifest/status/index append failures.
2. GREEN: plumb stderr writer and print warnings with error context.
3. REFACTOR: keep helper boundaries clear and avoid changing run success criteria.
4. GREEN: execute targeted and e2e checks.

## Notes

- Likely files: `cmd/tessariq/run.go` and associated tests for run command output/error handling.
- 2026-04-01T18:06:11Z: appendIndexEntry and appendRunningIndexEntry now emit warnings to stderr on manifest/status read and index append failures; unit tests cover all failure branches plus success path; e2e test confirms warning visible in full CLI run; CHANGELOG updated
