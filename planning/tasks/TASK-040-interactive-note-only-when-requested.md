---
id: TASK-040-interactive-note-only-when-requested
title: Gate interactive-without-attach note on explicit user request
status: completed
priority: low
spec_ref: specs/tessariq-v0.1.0.md#tessariq-run-task-path
dependencies:
    - TASK-018-replace-yolo-with-interactive-and-cli-polish
    - TASK-029-interactive-runtime-mode-independent-of-attach
updated_at: "2026-04-01T18:20:23Z"
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
