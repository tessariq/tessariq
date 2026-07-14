---
id: TASK-060-respect-grace-duration-during-container-shutdown
title: Respect configured grace periods during container shutdown escalation
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#agent-and-runtime-contract
dependencies:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-030-fix-timeout-signal-escalation
updated_at: "2026-04-03T08:55:06Z"
---

## Summary

`Process.Signal(SIGTERM)` currently shells out to `docker stop --time=10`, so Docker's hardcoded 10-second grace period wins and the runner's own `Config.Grace` timer never meaningfully controls escalation. Move the implementation back into Tessariq's own SIGTERM-then-SIGKILL ladder.

## Supersedes

- BUG-027 from `planning/BUGS.md`.

## Acceptance Criteria

- `--grace` determines the actual delay between SIGTERM and SIGKILL for container-backed runs.
- The runner log shows SIGKILL only when the configured grace period really expires.
- A process that exits promptly after SIGTERM never receives SIGKILL.
- The implementation does not bake in a second hardcoded Docker-side grace period that overrides Tessariq's config.

## Test Expectations

- Update unit tests for `signalCommand` so SIGTERM no longer maps to a fixed `docker stop --time=10` path.
- Add integration coverage for both graceful exit after SIGTERM and forced escalation after the configured grace timeout.
- Add regression coverage for interactive and non-interactive runner paths.

## TDD Plan

1. RED: add tests showing `--grace` is ignored with the current Docker command mapping.
2. GREEN: send a non-blocking SIGTERM to the container and let the runner's timer own escalation.
3. REFACTOR: keep shutdown behavior shared between interactive and non-interactive runner paths.

## Notes

- Likely files: `internal/container/process.go`, `internal/container/process_test.go`, `internal/runner/runner.go`, and runner integration tests.
- Accept either `docker kill --signal=SIGTERM` or an equivalent implementation, as long as `Config.Grace` becomes authoritative.
- 2026-04-03T08:55:06Z: Replaced docker stop --time=10 with docker kill --signal=SIGTERM for non-blocking signal delivery. Runner's Config.Grace timer now controls SIGTERM→SIGKILL escalation. Unit tests updated, integration test added for non-blocking behavior, manual tests confirmed all 4 acceptance criteria pass. Mutation testing: 85.74% efficacy.
