---
id: TASK-060-respect-grace-duration-during-container-shutdown
title: Respect configured grace periods during container shutdown escalation
status: todo
priority: p0
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-030-fix-timeout-signal-escalation
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-02T14:59:17Z"
areas:
    - runner
    - container
    - timeouts
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Signal-to-docker command mapping and grace-timer behavior are deterministic and heavily branch-driven.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Real container behavior is needed to prove SIGTERM then SIGKILL timing works as intended.
    e2e:
        required: false
        commands: []
        rationale: Focused integration coverage should be sufficient for the shutdown path.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Timeout and escalation logic is branch-heavy and safety-critical.
    manual_test:
        required: true
        commands: []
        rationale: Confirms user-facing `--grace` values match observed shutdown behavior.
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
