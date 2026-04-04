---
id: TASK-077-treat-terminal-non-success-run-outcomes-as-cli-failures
title: Treat terminal non-success run outcomes as CLI failures
status: todo
priority: p1
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-038-guaranteed-worktree-cleanup-on-run-failure
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#core-workflow
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-04T07:34:33Z"
areas:
    - cli
    - runner
    - lifecycle
    - ux
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Runner result shaping and CLI branching should start with precise unit coverage.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Terminal non-success paths cross real runner and process boundaries.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Exit code and printed-output semantics are directly user-visible CLI behavior.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Lifecycle branching is easy to partially fix while leaving inconsistent states behind.
    manual_test:
        required: true
        commands: []
        rationale: A real failed or timed-out run should confirm the final user-facing behavior and evidence guidance.
---

## Summary

`Runner.Run()` currently returns `nil` for ordinary terminal non-success states after it successfully writes `status.json`. The `run` command therefore prints success-style detached output and completes through its normal success path even when the run actually ended as `failed`, `timeout`, `killed`, or `interrupted`.

## Supersedes

- BUG-047 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq run` exits non-zero for terminal non-success outcomes after bootstrap, including failed, timeout, killed, and interrupted runs.
- Success-only detached output is printed only when the final run state is `success`.
- Non-success terminal outcomes still surface stable evidence details so users can inspect the failed run.
- The chosen behavior for workspace preservation versus cleanup remains explicit and consistent across terminal non-success paths.

## Test Expectations

- Add unit tests proving that failed and timed-out runs no longer flow through the success-only CLI output path.
- Add integration coverage for at least one non-success runner path that writes terminal status and then returns control to the CLI.
- Add e2e coverage for a real post-bootstrap failure or timeout showing non-zero exit status and non-success output semantics.
- Run mutation testing because CLI lifecycle branching and runner-result plumbing are safety-critical.

## TDD Plan

1. RED: add a failing test showing that a terminal `failed` status currently exits through the success path.
2. GREEN: plumb terminal run state back to the CLI so non-success outcomes do not masquerade as command success.
3. GREEN: align evidence guidance and workspace handling with the chosen non-success contract.
4. GREEN: rerun integration and e2e coverage for failed and timed-out runs.

## Notes

- Likely files: `internal/runner/runner.go`, `cmd/tessariq/run.go`, and run/runner integration or e2e tests.
- Keep this task focused on verified lifecycle semantics; do not fold speculative Ctrl+C context behavior into the implementation unless it is separately reproduced.
