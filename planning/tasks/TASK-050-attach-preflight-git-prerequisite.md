---
id: TASK-050-attach-preflight-git-prerequisite
title: Preflight git as an attach prerequisite
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#generated-runtime-state
dependencies:
    - TASK-007-attach-command-live-run-resolution
    - TASK-020-prerequisite-preflight-and-missing-dependency-ux
updated_at: "2026-04-02T07:35:44Z"
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
- 2026-04-02T07:35:44Z: Added DependencyGit to attach prerequisites. Unit tests (preflight_test.go, attach_test.go), e2e test (attach_e2e_test.go), and container manual tests all pass. CHANGELOG updated.
