---
id: TASK-005-runner-bootstrap-timeout-and-status-lifecycle
title: Implement runner bootstrap timeout handling and status lifecycle
status: todo
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
updated_at: 2026-03-29T12:06:20Z
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
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Timeout and terminal-state transitions are important mutation-test targets.
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
- Add integration tests for runner bootstrap and termination behavior using Testcontainers-backed collaborators only.
- E2E tests are deferred until attach and promote can observe the full lifecycle.
- Run mutation testing because lifecycle logic is safety-critical.

## TDD Plan

- Start with a failing unit test for timeout status emission, then a failing integration test for runner shutdown behavior.

## Notes

- Preserve durable evidence even on failure paths.
