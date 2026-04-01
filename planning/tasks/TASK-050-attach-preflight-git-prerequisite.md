---
id: TASK-050-attach-preflight-git-prerequisite
title: Preflight git as an attach prerequisite
status: todo
priority: p1
depends_on:
    - TASK-007-attach-command-live-run-resolution
    - TASK-020-prerequisite-preflight-and-missing-dependency-ux
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#generated-runtime-state
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-01T20:03:47Z"
areas:
    - cli
    - prerequisites
    - attach
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Command prerequisite sets and attach preflight behavior are best covered in unit tests.
    integration:
        required: false
        commands: []
        rationale: This fix is command-preflight logic and does not require additional collaborator coverage.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: The user-facing missing-prerequisite message should be validated on the actual command path.
    mutation:
        required: false
        commands: []
        rationale: This is a small prerequisite-list correction with limited branch complexity.
    manual_test:
        required: true
        commands: []
        rationale: Confirms attach now fails as a prerequisite error before repo-resolution work begins.
---

## Summary

`attach` resolves the current repository via `git`, but its prerequisite list only checks `tmux`. Add `git` to attach preflight so missing-host-tool failures happen in the spec-required prerequisite path.

## Supersedes

- BUG-018 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq attach <run-ref>` preflights both `tmux` and `git` before repo discovery.
- When `git` is missing or unavailable, attach fails with the standard prerequisite guidance rather than a raw exec error from `repoRoot()`.
- Existing missing-`tmux` guidance remains unchanged.
- No attach behavior changes once prerequisites pass.

## Test Expectations

- Add unit tests for `RequirementsForCommand("attach")` including both `git` and `tmux`.
- Add command-level or e2e coverage for `attach` with `git` absent from `PATH` and confirm actionable prerequisite messaging.
- Add regression coverage that successful attach flows are unchanged when both tools are available.

## TDD Plan

1. RED: add a failing test for attach prerequisites missing `git`.
2. GREEN: include `git` in the attach prerequisite set.
3. GREEN: verify attach fails before calling repository-root resolution when `git` is absent.

## Notes

- Likely files: `internal/prereq/preflight.go`, `cmd/tessariq/attach.go`, and attach command tests.
- Keep the shared prerequisite message format aligned with `TASK-020`.
