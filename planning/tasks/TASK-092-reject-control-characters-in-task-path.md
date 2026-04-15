---
id: TASK-092-reject-control-characters-in-task-path
title: Reject control characters in task path to prevent commit trailer injection
status: todo
priority: p2
depends_on:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-054-reject-symlinked-external-task-paths
    - TASK-058-reject-control-characters-in-allowlist-hosts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: ""
areas:
    - run
    - promote
    - evidence
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Path validation and trailer assembly are both table-driven and belong under deterministic unit coverage.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: The fix is pure validation plus string escaping; integration coverage is not required unless the promote pipeline changes.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Promote commit message composition is a user-visible contract and must be verified end to end.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Control-character predicates are trivial to weaken to a single-character check that still passes happy-path tests.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check should confirm that a task file whose name contains a newline is rejected at `tessariq run` and cannot produce forged commit trailers.
---

## Summary

Close a trailer-injection path on `tessariq promote` by rejecting newline and other ASCII control characters in the task-file path at `tessariq run`. `ValidateTaskPath` today only checks absoluteness, `.md` suffix, and symlink containment, so a file named `Fix: bug\nSigned-off-by: attacker@evil.example.md` is accepted. The path is stored verbatim in `manifest.task_path` and later interpolated into the `Tessariq-Task:` commit trailer by `promote.buildCommitMessage`, where the embedded newline splits into additional forged trailers. The filename-fallback branch of `ExtractTaskTitle` propagates the same value into `manifest.task_title`, allowing subject-line forgery as well.

## Supersedes

- BUG-057 from `planning/BUGS.md`.

## Acceptance Criteria

- `ValidateTaskPathLogic` rejects any task path containing a byte in `0x00..0x1F` or `0x7F`, producing a clear failure that matches the existing task-path validation contract.
- `ExtractTaskTitle` never returns a string containing a newline or other control character; the filename-fallback branch strips or refuses unsafe bytes so `manifest.task_title` is always trailer-safe.
- `promote.buildCommitMessage` either (a) refuses to promote when `manifest.task_path` or `manifest.task_title` contains a control character, producing a consistent failure-UX error, or (b) escapes the trailer value so git cannot parse injected lines as new trailers. The implementation picks exactly one strategy and pins it with tests.
- Normal task paths (spaces, Unicode, punctuation such as `:`, `?`, `!`, parentheses) continue to work unchanged.
- `tessariq run` fails fast with a user-facing error before creating any evidence artifacts when given a task path with a forbidden byte.
- Existing TASK-054 / TASK-058 control-character discipline is referenced so future additions to this class of check land in one place.

## Test Expectations

- Start with a failing unit test that calls `ValidateTaskPath` with a task path containing `\n` and asserts the new rejection error.
- Add unit coverage for `ExtractTaskTitle` proving the filename fallback never leaks control characters into the title.
- Add promote-level unit or integration coverage that reproduces the current trailer-injection attack and asserts the fix (either pre-validation or trailer escaping) prevents it.
- Add e2e coverage for a `tessariq run` attempt with a control-character task path that exits non-zero with a clear message and no evidence directory.
- Run mutation testing to pin the byte-range predicate.
- Manual test: create a file with a newline in its name, run `tessariq run <name>`, and verify rejection before worktree provisioning.

## TDD Plan

1. RED: add unit tests for the forbidden-byte predicate in `ValidateTaskPathLogic`.
2. RED: add a `promote` test that feeds a tampered manifest with an embedded newline in `task_path` and asserts the forged trailer does not reach the commit.
3. GREEN: implement the control-character rejection in `ValidateTaskPathLogic` and the filename-fallback branch of `ExtractTaskTitle`.
4. GREEN: add the defensive promote-time check (or trailer escaping) so tampered manifests also fail cleanly.
5. VERIFY: rerun the automated ladder plus the manual rejection check.

## Notes

- Prefer rejection over escaping at validation time — the task path is a filesystem path and should never legitimately contain control characters.
- Keep the defensive promote-time check minimal; it is a safety net against tampered manifests, not the primary gate.
- Align error text with the existing task-path error family so the failure UX is consistent.
