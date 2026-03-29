---
id: TASK-005-runner-bootstrap-timeout-and-status-lifecycle
title: Implement runner bootstrap timeout handling and status lifecycle
status: done
priority: p0
depends_on:
    - TASK-004-worktree-provisioning-and-workspace-metadata
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#core-workflow
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-03-29T20:09:22Z"
areas:
    - runner
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: State transitions, timeout bookkeeping, and artifact shaping belong in unit tests first.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Runner lifecycle needs real process coordination and integration coverage must use Testcontainers only.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: End-to-end flow should wait until attach and promote are connected.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Timeout and terminal-state transitions are important mutation-test targets.
    manual_test:
        required: true
        commands: []
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
---

## Summary

Implement bootstrap and runner lifecycle ownership for `status.json`, timeout handling, and core logs.

## Acceptance Criteria

- `status.json` exists even on bootstrap failure and is created before long-running runner work begins.
- Runner lifecycle produces exactly the v0.1.0 terminal states `success`, `failed`, `timeout`, `killed`, or `interrupted`, with valid `started_at` and `finished_at` timestamps.
- Timeout handling writes the expected evidence, including `timed_out` and `exit_code`, before escalation.
- `status.json` includes the minimum required fields for `schema_version`, `state`, `started_at`, `finished_at`, `exit_code`, and `timed_out`.
- Runner bootstrap records the deterministic container name `tessariq-<run_id>` in `manifest.json` before detached guidance prints it.
- `run.log` and `runner.log` remain durable even when bootstrap or timeout paths fail.

## Test Expectations

- Add unit tests for status transitions and timeout bookkeeping.
- Add unit tests for signal-to-state mapping: SIGTERM to `killed`, SIGINT to `interrupted`, and grace period expiration escalation (SIGTERM then SIGKILL).
- Add a unit test for deterministic container name derivation (`tessariq-<run_id>`), since downstream tasks depend on this contract.
- Add unit tests for `--pre` and `--verify` hook execution: CLI-order execution of multiple `--pre` values, pre-command failure halting the run before agent start, verify-command execution after agent completion, and verify-command failure affecting run status.
- Add integration tests for runner bootstrap and termination behavior using Testcontainers-backed collaborators only.
- Add integration tests for real process signal delivery and evidence durability when signals arrive during bootstrap or evidence writing.
- Add integration tests for `--pre`/`--verify` hook execution with real process boundaries.
- E2E tests are deferred until attach and promote can observe the full lifecycle.
- Run mutation testing because lifecycle logic is safety-critical.

## TDD Plan

- Start with a failing unit test for timeout status emission, then a failing integration test for runner shutdown behavior.

## Notes

- Preserve durable evidence even on failure paths.
- 2026-03-29T20:09:22Z: All ACs met. status.json with 6 required fields, 5 terminal states, timeout.flag+escalation, container_name in manifest.json, durable run.log+runner.log. Unit tests (11 runner, 10 status, 3 signal, 3 timeout, 5 logs, 8 hooks, 3 container), integration tests (7 runner, 4 hooks), mutation efficacy 91.43%. Manual test: 6/6 pass. Evidence: planning/artifacts/manual-test/TASK-005-runner-bootstrap-timeout-and-status-lifecycle/20260329T200616Z/report.md, planning/artifacts/verify/task/TASK-005-runner-bootstrap-timeout-and-status-lifecycle/20260329T200548Z/report.json
