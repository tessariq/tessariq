---
id: TASK-071-implement-run-attach-live-session
title: Make `tessariq run --attach` attach to the live tmux session
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#tessariq-run-task-path
dependencies:
    - TASK-006-tmux-session-and-detached-attach-guidance
    - TASK-007-attach-command-live-run-resolution
    - TASK-029-interactive-runtime-mode-independent-of-attach
updated_at: "2026-04-05T11:17:44Z"
---

## Summary

`--attach` is exposed on `tessariq run` but currently has no behavior beyond suppressing a note. Wire the flag into the run lifecycle so the command attaches to the run's tmux session once it is ready.

## Supersedes

- BUG-037 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq run --attach <task-path>` attaches the invoking terminal to the newly created tmux session for that run.
- Detached-by-default behavior remains unchanged when `--attach` is not set.
- `--interactive --attach` uses the same live session path instead of leaving the run detached.
- Attach failures surface actionable errors without silently falling back to detached mode.

## Test Expectations

- Add unit tests for run/runner control flow that prove `--attach` reaches the tmux attach path only after session creation succeeds.
- Add integration coverage for session creation plus attach invocation using containerized tmux collaborators only.
- Add a thin e2e regression that proves `run --attach` does not exit detached.
- Run mutation testing because partial wiring could leave the flag superficially present but still ineffective.

## TDD Plan

1. RED: add a failing unit test that proves `cfg.Attach` currently has no effect on the run lifecycle.
2. GREEN: surface session readiness and foreground attach at the CLI/runner boundary.
3. GREEN: add integration and e2e coverage for the attached run path.

## Notes

- Likely files: `cmd/tessariq/run.go`, `internal/runner/runner.go`, and `internal/tmux/tmux.go`.
- Prefer the smallest change that preserves detached runs and existing `tessariq attach` behavior.
- 2026-04-05T11:17:44Z: Wired --attach flag into run lifecycle via SessionReady channel on Runner; runner signals after tmux session creation, cmd layer attaches terminal concurrently. Unit tests: 3 SessionReady + 4 runWithAttach + 1 output suppression. Integration test: 1 real tmux session ready. E2e test: 1 container-mode attach path. Manual tests: 4/4 pass. Mutation efficacy: 85.87%. Evidence: (evidence artifacts; path omitted)
