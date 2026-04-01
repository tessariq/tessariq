---
id: TASK-047-promote-repo-local-evidence-path-validation
title: Reject non-repo evidence paths during promote resolution
status: todo
priority: p0
depends_on:
    - TASK-014-run-index-and-run-ref-resolution
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-045-validate-index-entry-shape-before-resolution
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#core-workflow
    - specs/tessariq-v0.1.0.md#generated-runtime-state
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-01T20:03:47Z"
areas:
    - promote
    - evidence
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Evidence-path normalization and repo-boundary rejection are deterministic path rules.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Promote should be exercised against real git side effects and forged index inputs.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: This is a user-visible safety boundary on a primary workflow.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Path-validation branches should survive mutation testing.
    manual_test:
        required: true
        commands: []
        rationale: Confirms forged external evidence cannot be promoted into a real branch and commit.
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
