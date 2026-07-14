---
id: TASK-029-interactive-runtime-mode-independent-of-attach
title: Implement interactive runtime mode independently of attach
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#tessariq-run-task-path
dependencies:
    - TASK-018-replace-yolo-with-interactive-and-cli-polish
    - TASK-027-container-lifecycle-and-mount-isolation
    - TASK-028-container-session-streaming-and-cleanup-hardening
updated_at: "2026-03-31T17:33:11Z"
---

## Summary

Implement true interactive run support as a runtime feature in its own right, without coupling the `--interactive` contract to the `attach` command surface.

## Acceptance Criteria

- `tessariq run --interactive` starts the selected agent in a mode that can receive human approval input through a live terminal path.
- Interactive mode remains logically independent from `attach`; users can adopt different terminal workflows without changing the flag contract.
- Detached runs still preserve the durable evidence and host tmux session contracts.
- Timeout behavior for interactive runs is explicit and tested; the active-agent timeout pauses while the run is waiting for human approval instead of silently forcing an arbitrary very long default timeout.
- Interactive runs fail cleanly with actionable guidance when the selected agent or runtime image cannot support the required terminal behavior.
- Agent metadata records requested versus applied interactive behavior accurately.

## Test Expectations

- Add unit tests for interactive runtime validation and timeout/approval-state transitions, including pausing active-agent timeout accounting during human approval waits.
- Add integration tests for interactive terminal wiring across tmux and container boundaries.
- Add e2e coverage for an interactive run that receives approval input successfully.
- Add e2e coverage for unsupported interactive-runtime failure paths.
- Run mutation testing because interactive lifecycle handling is branch-heavy and user-visible.

## TDD Plan

- Start with a failing unit test that proves active-agent timeout accounting pauses during human approval waits, then add a failing integration test for terminal wiring before implementation.

## Notes

- Prefer explicit interactive timeout semantics over silently replacing the normal timeout with a very large default.
- The baseline contract for this task is: pause active-agent timeout accounting while the run is waiting for human approval.
- If an overall wall-clock guard is needed in addition to the paused active-agent timeout, it must be explicit and separately documented rather than implicit.
- `tessariq attach` may remain a useful live-run entry point, but it must not be the sole conceptual owner of interactive mode.
- 2026-03-31T17:33:11Z: Interactive runtime mode implemented: container TTY support (-i -t flags), activity-based timeout that pauses during idle periods, agent-specific validation (opencode rejects, claude-code accepts), tmux session uses docker attach for interactive runs. Unit tests: 6 activity timer + 4 runner interactive + 2 container. Integration: all pass. E2e: 3 new interactive tests (opencode rejection, claude-code acceptance, agent metadata). Mutation: 83.28% efficacy (>70% threshold). Local-only manual-test artifacts generated.
