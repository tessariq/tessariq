---
id: TASK-086-make-diff-artifact-writes-all-or-nothing
title: Make diff artifact writes all-or-nothing for promotable runs
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#tessariq-run-task-path
dependencies:
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-049-promote-require-diffstat-for-changed-runs
    - TASK-057-surface-diff-artifact-write-errors
updated_at: "2026-04-14T14:35:51Z"
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
- 2026-04-14T14:35:51Z: Atomic tmp+rename writer with rollback (internal/runner/diff.go); Runner.DiffArtifactWriter hook escalates diff write failure to StateFailed; CLI wires closure; manual test report: (evidence artifacts; path omitted) verify clean (hybrid)
