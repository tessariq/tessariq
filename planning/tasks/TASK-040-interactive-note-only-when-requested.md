---
id: TASK-040-interactive-note-only-when-requested
title: Gate interactive-without-attach note on explicit user request
status: done
priority: p2
depends_on:
    - TASK-018-replace-yolo-with-interactive-and-cli-polish
    - TASK-029-interactive-runtime-mode-independent-of-attach
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-01T18:20:23Z"
areas:
    - cli
    - runtime
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: The note gating condition is a small decision branch and should be covered with focused unit tests.
    integration:
        required: false
        commands: []
        rationale: Behavior is command-output logic and does not require container collaborators.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Validate user-visible stderr guidance for default vs explicitly interactive runs.
    mutation:
        required: false
        commands: []
        rationale: Existing mutation coverage can remain unchanged for this targeted output fix.
    manual_test:
        required: true
        commands: []
        rationale: Confirm CLI UX wording and absence/presence of the note in real runs.
---

## Summary

Default Claude Code runs currently emit an interactive-without-attach note even when `--interactive` was not requested. Restrict the note to explicit `--interactive` requests so default UX is quiet and accurate.

## Supersedes

- BUG-008 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq run` (default Claude Code settings, no `--interactive`) does not print the interactive-without-attach note.
- `tessariq run --interactive` without `--attach` still prints the note with session guidance.
- Existing attach behavior and runtime mode selection remain unchanged.
- Automated tests assert both note-suppressed and note-emitted paths.

## Test Expectations

- Add or update command-level tests for note gating with these cases:
  - default run config (`interactive=false`, `attach=false`) => no note.
  - explicit interactive (`interactive=true`, `attach=false`) => note emitted.
  - interactive + attach => no detached guidance note.
- Add/update an e2e assertion that default runs do not include the detached interactive warning.

## TDD Plan

1. RED: add failing test demonstrating default runs incorrectly emit the note.
2. GREEN: change the condition to gate on explicit config intent.
3. REFACTOR: keep note text and session naming untouched while minimizing behavior changes.
4. GREEN: run targeted and e2e verification.

## Notes

- Likely files: `cmd/tessariq/run.go` and relevant command/e2e test files.
- 2026-04-01T18:20:23Z: fix: gate interactive note on cfg.Interactive instead of Applied[interactive]; unit tests for 3 flag combos + e2e assertions for both paths; manual test verdict: pass
