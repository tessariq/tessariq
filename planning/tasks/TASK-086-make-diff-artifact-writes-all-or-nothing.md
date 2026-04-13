---
id: TASK-086-make-diff-artifact-writes-all-or-nothing
title: Make diff artifact writes all-or-nothing for promotable runs
status: todo
priority: p1
depends_on:
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-049-promote-require-diffstat-for-changed-runs
    - TASK-057-surface-diff-artifact-write-errors
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#success-metrics
updated_at: "2026-04-13T20:17:33Z"
areas:
    - runner
    - evidence
    - promote
    - lifecycle
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: The smallest regression here is deterministic diff-artifact writer behavior and run-result shaping.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The bug spans real git diff generation, evidence writes, and promote completeness checks.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: The user-visible regression is a successful run that later cannot be promoted.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Partial-write and terminal-state branches are easy to weaken with a superficial fix.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check should confirm that a changed run never ends in a misleadingly promotable success shape.
---

## Summary

Fix the diff-artifact write path so a changed run never finishes with only one of `diff.patch` or `diffstat.txt` present.

## Supersedes

- BUG-049 from `planning/BUGS.md`.

## Acceptance Criteria

- For runs with code changes, Tessariq either writes both `diff.patch` and `diffstat.txt` as non-empty evidence artifacts or does not report the run as a normal successful, promotable completion.
- A failure while writing the second diff artifact must not leave an orphan `diff.patch` behind as if the evidence set were intact.
- The chosen behavior remains internally consistent: either both diff artifacts are committed atomically or the run ends in a non-success outcome with clear evidence guidance.
- `tessariq promote` must no longer encounter a normal successful run whose evidence is missing only one of the required diff artifacts.

## Test Expectations

- Start with a failing unit test around the diff-artifact writer showing that a second-write failure currently leaves partial evidence behind.
- Add integration coverage that exercises a changed run or direct diff-artifact generation path and proves partial evidence is cleaned up or escalated.
- Add e2e or high-level CLI coverage showing that a run with diff-artifact write failure does not masquerade as a clean success that later fails at promote time.
- Run mutation testing because the fix touches evidence completeness and terminal lifecycle semantics.

## TDD Plan

1. RED: reproduce the partial-write case where `diff.patch` exists but `diffstat.txt` does not.
2. GREEN: make the smallest fix so changed-run diff artifacts become all-or-nothing from the caller's perspective.
3. GREEN: align run-result handling so successful runs remain promotable by construction.
4. VERIFY: rerun promote-oriented automated coverage and manual testing.

## Notes

- Keep this task focused on pairwise diff-artifact integrity; do not fold unrelated proxy evidence completeness changes into the same fix.
- A valid implementation may use tmp+rename, best-effort rollback of the first file on second-write failure, or terminal failure escalation, as long as successful runs no longer carry self-contradictory diff evidence.
