---
id: TASK-047-promote-repo-local-evidence-path-validation
title: Reject non-repo evidence paths during promote resolution
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#core-workflow
dependencies:
    - TASK-014-run-index-and-run-ref-resolution
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-045-validate-index-entry-shape-before-resolution
updated_at: "2026-04-02T08:38:48Z"
---

## Summary

`promote` currently trusts `index.jsonl` `evidence_path` values verbatim. Enforce that the resolved evidence directory is exactly repo-local run evidence under `<repo>/.tessariq/runs/<run_id>` before any evidence is read or any git side effects occur.

## Supersedes

- BUG-013 from `planning/BUGS.md`.

## Acceptance Criteria

- `promote` rejects absolute `evidence_path` values from the index.
- Relative `evidence_path` values are cleaned and rejected if they escape the repository root or `.tessariq/runs/` subtree.
- Promotion fails before reading manifest, status, or patch data when the evidence path is not repo-local for the resolved run.
- Failure messaging makes clear that the referenced run evidence is invalid or outside the repository.

## Test Expectations

- Add unit tests for absolute-path rejection, `..` escape rejection, and acceptance of the canonical `.tessariq/runs/<run_id>` path.
- Add integration or e2e adversarial coverage with a forged index entry pointing at external evidence and assert no branch or commit is created.
- Add a regression test that relative repo-local evidence still promotes normally.

## TDD Plan

1. RED: add a failing test for an absolute external `evidence_path`.
2. GREEN: normalize and validate evidence paths against the repo-local runs directory.
3. REFACTOR: keep validation close to run-ref resolution so later promote steps only see trusted evidence paths.
4. GREEN: verify promote exits before any git side effects on invalid evidence.

## Notes

- Likely files: `internal/promote/promote.go` and promote integration/e2e tests.
- Prefer a shared validation helper if attach needs the same repo-boundary enforcement.
- 2026-04-02T08:38:48Z: Evidence path validation added; unit/integration/e2e/mutation/manual tests all pass
