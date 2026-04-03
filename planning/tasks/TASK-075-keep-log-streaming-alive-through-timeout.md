---
id: TASK-075-keep-log-streaming-alive-through-timeout
title: Keep run log streaming alive through timeout and grace-period shutdown
status: todo
priority: p3
depends_on:
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-028-container-session-streaming-and-cleanup-hardening
    - TASK-060-respect-grace-duration-during-container-shutdown
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#required-artifacts
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-03T12:31:03Z"
areas:
    - container
    - runner
    - logging
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Context ownership and log-drain control flow should begin with focused unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Timeout-era log capture must be exercised against a real container lifecycle.
    e2e:
        required: false
        commands: []
        rationale: Focused integration coverage should be sufficient for the timeout log-drain behavior.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Timeout and log-drain paths are subtle and easy to partially fix.
    manual_test:
        required: true
        commands: []
        rationale: "A real timeout scenario should confirm grace-period shutdown output reaches `run.log`."
---

## Summary

`docker logs --follow` is currently tied to the timeout context, so the log follower dies before the container finishes graceful shutdown. Keep log streaming alive until the container actually exits so timeout evidence includes the final shutdown output.

## Supersedes

- BUG-041 from `planning/BUGS.md`.

## Acceptance Criteria

- `run.log` includes agent output emitted after the timeout fires but before the container exits during the grace period.
- The log follower is cancelled only after container exit or explicit teardown, not immediately when the timeout context is cancelled.
- Normal successful runs still stream and drain logs correctly.
- The fix does not introduce leaked goroutines or hanging waits after the container is gone.

## Test Expectations

- Add unit tests around the log-streaming context ownership and drain behavior.
- Add integration coverage for a container that emits output during graceful shutdown after timeout.
- Add regression coverage that normal exit and immediate start-failure paths still close log streaming cleanly.
- Run mutation testing because timeout and log-drain control flow is safety-critical.

## TDD Plan

1. RED: add a failing test that proves post-timeout shutdown output is missing from `run.log`.
2. GREEN: decouple log streaming from the timeout context while keeping cleanup deterministic.
3. GREEN: rerun timeout and normal-exit coverage to confirm logs still drain fully.

## Notes

- Likely files: `internal/container/process.go`, `internal/runner/runner.go`, and timeout/log integration tests.
- Prefer a dedicated log-streaming context or explicit cancel function over broadening the timeout context lifetime.
