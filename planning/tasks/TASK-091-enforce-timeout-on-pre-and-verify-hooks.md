---
id: TASK-091-enforce-timeout-on-pre-and-verify-hooks
title: Enforce run timeout and grace on pre and verify hooks
status: done
priority: p1
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-030-fix-timeout-signal-escalation
    - TASK-060-respect-grace-duration-during-container-shutdown
    - TASK-072-run-hooks-from-repo-root
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-15T12:20:18Z"
areas:
    - runner
    - hooks
    - lifecycle
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Timeout bookkeeping across the run phases belongs under deterministic unit coverage.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Hook cancellation must propagate through `exec.CommandContext` and escalate via real signal delivery.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: The --timeout flag is a user-visible SLA and should be validated from a CLI-level pre-hook scenario.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Timeout accounting is easy to weaken to a shape that still passes happy-path tests.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check should confirm that a hung pre-hook is killed and the run reaches a terminal state within the declared timeout budget.
---

## Summary

Make `--timeout` and `--grace` bound the entire run, not just the agent process. Today `Runner.Run` calls `RunPreHooks` and `RunVerifyHooks` with the top-level CLI context and no deadline of their own, so a pre or verify hook that hangs (a stalled HTTP call, a `sleep infinity`, a deadlocked script) pins the run indefinitely — no `timeout.flag` is written, no terminal `status.json` is produced, and the user only recovers by killing the CLI process. Users rely on `--timeout` as an unattended-run SLA; hooks must honor it.

## Supersedes

- BUG-056 from `planning/BUGS.md`.

## Acceptance Criteria

- `RunPreHooks` and `RunVerifyHooks` cannot outlive the run's `--timeout` budget. A hook that would exceed the remaining budget is cancelled via `exec.CommandContext`, and the runner records a terminal state describing the hook-induced timeout.
- The budget is accounted against `cfg.Timeout` across all phases: pre-hooks, agent process, and verify hooks together may not consume more than `cfg.Timeout`. The implementation must make clear in `runner.log` which phase exhausted the budget.
- On hook timeout, the runner writes `timeout.flag` and a `StateTimeout` (or an equivalent, clearly named terminal state) in `status.json`, with `runner.log` identifying the offending hook command by index.
- After sending cancellation to a timed-out hook, the runner honors `--grace` before escalating (matching the TASK-030/TASK-060 escalation discipline for the agent process).
- Pre-hook timeout must still run the existing cleanup defers (worktree, runtime-state, proxy topology) so the host is left in a clean state.
- Normal runs where hooks complete within the budget remain unaffected; existing hook stdout/stderr capture, CWD handling (TASK-072), and failure propagation are preserved.

## Test Expectations

- Start with a failing unit test that configures a pre-hook slower than the declared timeout and asserts the runner produces a terminal state plus `timeout.flag` within the budget.
- Add symmetric coverage for a timed-out verify hook.
- Add integration coverage proving real `sh` child processes are actually killed on timeout (not just marked cancelled in Go state).
- Add CLI-level e2e coverage for `tessariq run --timeout <short> --pre 'sleep <long>'` exiting with the documented failure UX and full evidence artifacts.
- Run mutation testing because conditional timeout accounting is easy to weaken.
- Manual test: build the CLI and verify a hung pre-hook surfaces `evidence_path`, prints the failure banner, and exits non-zero inside the timeout budget.

## TDD Plan

1. RED: add a unit test for a pre-hook that runs longer than the remaining budget and assert terminal state + `timeout.flag`.
2. RED: add the equivalent verify-hook test.
3. GREEN: thread a hook-specific `context.WithTimeout` derived from remaining budget into `runHook`, with grace-period escalation matching the agent-process escalation path.
4. GREEN: update `runner.log` and terminal-status writing so the user can distinguish a hook timeout from an agent timeout.
5. REFACTOR: centralize the "remaining budget" accounting in one helper so both phases and future phases share the same logic.
6. VERIFY: rerun the full automated ladder and the manual long-hook scenario.

## Notes

- Keep `--grace` semantics consistent with the agent-process path: send SIGTERM first, then SIGKILL after the grace duration.
- Do not add a new user-facing flag for hook timeouts in v0.1.0. The single `--timeout` budget is sufficient and matches the spec's SLA story.
- The existing signal escalation module (`internal/runner/signal.go`) is the correct location for shared escalation helpers.
- 2026-04-15T12:20:18Z: hook budget enforced across pre/verify; SIGTERM->grace->SIGKILL; shared run deadline; unit+integration+e2e+manual coverage; verify clean
