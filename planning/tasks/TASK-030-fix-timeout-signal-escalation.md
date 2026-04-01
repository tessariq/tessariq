---
id: TASK-030-fix-timeout-signal-escalation
title: Fix timeout signal escalation to send SIGTERM before SIGKILL
status: done
priority: p1
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-028-container-session-streaming-and-cleanup-hardening
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#runner-responsibilities
updated_at: "2026-04-01T10:57:36Z"
areas:
    - runner
    - container
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Signal escalation sequence is branch-heavy and must be verified with unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Real container signal behavior crosses process boundaries.
    e2e:
        required: false
        commands: []
        rationale: Existing e2e timeout coverage from TASK-005 is sufficient once unit/integration pass.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Signal escalation logic is safety-critical and easy to weaken.
    manual_test:
        required: false
        commands: []
        rationale: Signal behavior is fully testable through automated tests.
---

## Summary

TASK-005 acceptance criteria specify "grace period expiration escalation (SIGTERM then SIGKILL)" but the current implementation sends `os.Kill` (SIGKILL) immediately on timeout without attempting graceful shutdown via SIGTERM first. This task fixes the escalation sequence to match the spec and TASK-005's stated behavior.

## Supersedes

This task addresses a gap in TASK-005's implementation. TASK-005 is done and its acceptance criteria are correct; the implementation diverged from the stated escalation pattern.

## Acceptance Criteria

- On timeout, the runner sends SIGTERM (via `docker stop`) to the container first, not SIGKILL.
- After the configured grace period expires without the container exiting, the runner escalates to SIGKILL (via `docker kill`).
- `timeout.flag` is written before the first signal (SIGTERM), not after.
- The terminal state for a timed-out run remains `timeout` regardless of which signal ultimately stops the container.
- The `status.json` `timed_out` field is `true` for both graceful and forced timeout exits.

## Test Expectations

- Add unit tests for the two-step escalation: SIGTERM first, SIGKILL after grace period expiration.
- Add unit tests verifying that a container that exits promptly after SIGTERM does not receive SIGKILL.
- Add unit tests for `timeout.flag` being written before the first signal.
- Add integration tests for timeout escalation using Testcontainers-backed collaborators only.
- Run mutation testing because signal escalation logic is safety-critical.

## TDD Plan

1. RED: write unit test asserting SIGTERM is sent before SIGKILL on timeout.
2. RED: write unit test asserting SIGKILL is not sent when container exits promptly after SIGTERM.
3. RED: write unit test asserting `timeout.flag` is written before the first signal.
4. GREEN: implement two-step escalation in `runProcess` timeout path.
5. IMPROVE: refactor signal escalation into a testable helper if warranted.
6. RED: write integration test for timeout escalation using Testcontainers.
7. GREEN: verify integration test passes against real Docker.

## Notes

- Files likely affected: `internal/runner/runner.go` (timeout handling), `internal/runner/runner_test.go`, `internal/runner/runner_integration_test.go`.
- The existing `docker stop` command already sends SIGTERM with a configurable grace period before SIGKILL — verify whether the current implementation already uses `docker stop` or sends `os.Kill` directly.
- 2026-04-01T10:57:36Z: Implemented two-step timeout signal escalation (SIGTERM then SIGKILL). 4 unit tests, 2 integration tests, 5 manual tests all pass. Mutation testing at 85.15% efficacy.
