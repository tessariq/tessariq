---
id: TASK-029-interactive-runtime-mode-independent-of-attach
title: Implement interactive runtime mode independently of attach
status: todo
priority: p0
depends_on:
    - TASK-018-replace-yolo-with-interactive-and-cli-polish
    - TASK-027-container-lifecycle-and-mount-isolation
    - TASK-028-container-session-streaming-and-cleanup-hardening
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-31T13:20:00Z"
areas:
    - container
    - tmux
    - interactive
    - timeout
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Interactive runtime state handling and timeout semantics should start with focused unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Interactive mode depends on real terminal and process behavior across host tmux and Docker boundaries.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Interactive runtime is a primary user-visible workflow and needs thin end-to-end coverage.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Timeout and approval-state branching should survive mutation testing.
    manual_test:
        required: true
        commands: []
        rationale: Human approval flows must be exercised manually with a live interactive run.
---

## Summary

Implement true interactive run support as a runtime feature in its own right, without coupling the `--interactive` contract to the `attach` command surface.

## Acceptance Criteria

- `tessariq run --interactive` starts the selected agent in a mode that can receive human approval input through a live terminal path.
- Interactive mode remains logically independent from `attach`; users can adopt different terminal workflows without changing the flag contract.
- Detached runs still preserve the durable evidence and host tmux session contracts.
- Timeout behavior for interactive runs is explicit and tested; approval wait time must not silently force an arbitrary very long default timeout.
- Interactive runs fail cleanly with actionable guidance when the selected agent or runtime image cannot support the required terminal behavior.
- Agent metadata records requested versus applied interactive behavior accurately.

## Test Expectations

- Add unit tests for interactive runtime validation and timeout/approval-state transitions.
- Add integration tests for interactive terminal wiring across tmux and container boundaries.
- Add e2e coverage for an interactive run that receives approval input successfully.
- Add e2e coverage for unsupported interactive-runtime failure paths.
- Run mutation testing because interactive lifecycle handling is branch-heavy and user-visible.

## TDD Plan

- Start with a failing unit test for the chosen interactive timeout semantics, then add a failing integration test for terminal wiring before implementation.

## Notes

- Prefer explicit interactive timeout semantics over silently replacing the normal timeout with a very large default.
- This task should evaluate separate timers for active agent runtime versus human approval wait, or another equally explicit contract.
- `tessariq attach` may remain a useful live-run entry point, but it must not be the sole conceptual owner of interactive mode.
