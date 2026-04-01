---
id: TASK-039-capped-log-writers-and-truncation-markers
title: Enforce capped run logs with explicit truncation markers
status: done
priority: p0
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-028-container-session-streaming-and-cleanup-hardening
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: "2026-04-01T09:23:30Z"
areas:
    - runner
    - logs
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Cap/truncation behavior should be deterministic at writer boundary level.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Container log streaming path must be exercised with real process output.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: User-visible evidence files must match capped-log contract under realistic runs.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Boundary and marker logic is branch-heavy and benefits from mutation checks.
    manual_test:
        required: true
        commands: []
        rationale: Confirm truncation UX in emitted artifacts with intentionally noisy runs.
---

## Summary

Current run log handling streams unbounded output into evidence files without an explicit truncation marker. This task introduces capped log writers so `run.log` and `runner.log` are bounded and visibly marked when truncated.

## Supersedes

- BUG-007 from `planning/BUGS.md`.

## Acceptance Criteria

- `run.log` and `runner.log` are capped to a deterministic maximum size.
- When cap is reached, truncation marker text is written so consumers can distinguish truncated logs from complete logs.
- Log writes after cap do not grow files beyond configured limit.
- Existing detached and interactive log streaming behavior continues to function.
- Capping strategy is documented and validated by automated tests.

## Test Expectations

- Add unit tests for capped writer behavior including exact-boundary and over-boundary writes.
- Add unit tests for one-time truncation marker insertion and idempotence.
- Add integration tests generating oversized container output and asserting file size cap + marker.
- Add e2e regression test validating capped evidence logs on a noisy run.

## TDD Plan

1. RED: add failing writer-level tests for cap enforcement and truncation marker behavior.
2. RED: add failing runner/container integration test for oversized output.
3. GREEN: implement capped writer abstraction and wire into log setup/streaming.
4. REFACTOR: keep cap configuration centralized and discoverable.
5. GREEN: validate across detached and interactive modes.

## Notes

- Likely files: `internal/runner/logs.go`, `internal/runner/runner.go`, `internal/container/process.go`, and associated tests.
- Ensure marker insertion does not race with concurrent stdout/stderr writes.
- 2026-04-01T09:23:30Z: Implemented write-time CappedWriter enforcing log size limits during execution. CappedWriter wraps io.Writer with mutex-protected byte tracking, appends truncation marker at cap boundary, and silently discards subsequent writes. Wired into LogFiles so both run.log and runner.log are capped at 50 MiB. Removed post-run CapLogFile calls. All unit/integration/e2e/mutation/manual tests pass.
