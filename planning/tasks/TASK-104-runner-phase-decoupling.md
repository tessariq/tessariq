---
id: TASK-104-runner-phase-decoupling
title: Reduce Runner.Run phase coupling with typed phase results
status: blocked
priority: low
dependencies:
    - TASK-017-v0-1-0-spec-conformity-closeout
milestone: v0.2.0
spec_version: v0.2.0
spec_ref: specs/tessariq-v0.2.0.md#runner-responsibilities
spec_refs:
    - specs/tessariq-v0.2.0.md#runner-responsibilities
    - specs/tessariq-v0.2.0.md#lifecycle-rules
    - specs/tessariq-v0.2.0.md#resume-rules
updated_at: "2026-06-10T00:00:00Z"
areas:
    - runner
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Each extracted phase must be unit-testable in isolation with injected collaborators while preserving downgrade and reconciliation rules.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The runner lifecycle spans tmux, container, and evidence boundaries; the refactor must not break those contracts.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Runner.Run is on the critical run path; terminal-state and timeout behavior must stay green end-to-end.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Status downgrade, timeout-flag, and signal-derived-state branches are subtle correctness logic and must survive mutation.
    manual_test:
        required: true
        commands: []
        rationale: Confirms status.json terminal states for success, failure, timeout, and cleanup-error runs are unchanged.
---

## Summary

Reduce coupling inside `internal/runner/runner.go`'s `Runner.Run` by extracting phase-level behavior into small private methods that return typed phase results. Keep the lifecycle explicit and the state transitions visible — this is not a move to a broad state-machine abstraction.

## Motivation

`Runner.Run` is explicit and carefully tested, but it combines initial evidence writes, log setup, tmux session setup, pre-hooks, detached/interactive process execution, timeout handling, verify hooks, diff artifacts, cleanup, and terminal status reconciliation in one dense function. The downgrade rules (preserve failed state when diff-artifact writing fails, cleanup-failure downgrades, timeout-flag semantics, signal-derived states) are easy to break accidentally. v0.2.0 adds resume rules and workspace-mode-specific lifecycle behavior (`specs/tessariq-v0.2.0.md#resume-rules`, `#lifecycle-rules`) that will touch this code, so reducing phase coupling now lowers the risk of those additions.

This is review finding #2 and review priority #4. The review is explicit: keep `Runner.Run` explicit; only isolate phases where it reduces coupling.

## Acceptance Criteria

- Distinct lifecycle phases (e.g. bootstrap/evidence init, session start, agent execution, timeout enforcement, verify, diff artifacts, cleanup, terminal reconciliation) are extracted into small private methods that each return a typed result rather than mutating shared state in place.
- `Runner.Run` becomes an orchestration spine that sequences phases and applies the terminal-state reconciliation rules in one readable place.
- All existing downgrade and reconciliation semantics are preserved exactly: failed states survive diff-artifact write failure, cleanup failures downgrade as today, timeout writes `timeout.flag` before escalation, and signal-derived terminal states are unchanged.
- Each extracted phase has at least one focused unit test exercising its success and primary failure branch with injected collaborators.
- `internal/runner` tests pass with behavior assertions unchanged; integration and e2e run paths stay green.

## Non-Goals

- No broad state-machine framework or generic phase abstraction.
- No change to `status.json` states, `timeout.flag` semantics, or evidence outputs.
- No reordering of lifecycle phases or change to which evidence is written when.

## Test Expectations

- Each extracted phase gets a focused unit test covering its success and primary failure branch with injected collaborators.
- A table-driven test pins the full terminal-state downgrade matrix (success, failure, timeout, cleanup-error, signal-derived).
- Integration and e2e run paths confirm `status.json` terminal states and `timeout.flag` semantics are unchanged.
- Mutation testing guards the downgrade, timeout, and signal-derived branches.

## TDD Plan

- For each phase, RED: add a unit test pinning its current observable result (returned status/error and any evidence side effect) before extraction.
- GREEN: extract the phase into a private method returning a typed result; wire it into `Run`.
- REFACTOR: once phases return typed results, consolidate the terminal-state reconciliation into a single explicit decision step and assert the full downgrade matrix with a table-driven test.

## Notes

- Blocked until `v0.2.0` becomes the active milestone; depends on the v0.1.0 closeout (`TASK-017`).
- Scope this alongside, and ideally just before, the v0.2.0 resume/lifecycle feature work so those additions land on the decoupled phases.
- Keep the diff small and behavior-preserving; this is a coupling-reduction task, not a redesign.
