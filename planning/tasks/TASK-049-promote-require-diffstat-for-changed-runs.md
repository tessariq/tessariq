---
id: TASK-049-promote-require-diffstat-for-changed-runs
title: Require diffstat.txt when promoting changed runs
status: todo
priority: p1
depends_on:
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-01T20:03:47Z"
areas:
    - promote
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Evidence completeness rules should be tightened with focused unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Promote evidence checks should be verified against real finished-run fixtures.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Missing-artifact behavior is user-visible on the promote command.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Required-artifact branching is safety-critical and easy to weaken accidentally.
    manual_test:
        required: true
        commands: []
        rationale: Confirms changed runs with missing `diffstat.txt` are rejected with actionable guidance.
---

## Summary

The spec requires both `diff.patch` and `diffstat.txt` when a run has changes, but promote currently only enforces `diff.patch`. Extend evidence completeness so changed runs cannot promote when `diffstat.txt` is missing.

## Supersedes

- BUG-014 from `planning/BUGS.md`.

## Acceptance Criteria

- Finished runs with code changes fail promote if `diffstat.txt` is missing or empty.
- Failure guidance identifies `diffstat.txt` as the missing required artifact.
- Zero-diff runs still follow the existing no-code-changes path instead of being forced to produce diff artifacts.
- Evidence completeness checks remain compatible with unchanged runs that legitimately have no diff artifacts.

## Test Expectations

- Add unit tests for completeness behavior when `diff.patch` exists but `diffstat.txt` is missing.
- Add integration or e2e regression for promote failure on missing `diffstat.txt` with a non-empty patch.
- Add regression coverage for valid changed runs and legitimate zero-diff runs.

## TDD Plan

1. RED: add a failing test for changed evidence missing `diffstat.txt`.
2. GREEN: require `diffstat.txt` whenever the run has changes and promote expects diff artifacts.
3. REFACTOR: keep evidence-completeness logic explicit about always-required vs changed-run-required files.
4. GREEN: verify error messaging names the missing artifact.

## Notes

- Likely files: `internal/runner/completeness.go`, `internal/promote/promote.go`, and promote tests.
- Preserve the existing zero-diff guard contract from `TASK-015`.
