---
id: TASK-094-reconcile-terminal-status-with-container-cleanup-outcome
title: Reconcile terminal status with container cleanup outcome
status: done
priority: p2
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-028-container-session-streaming-and-cleanup-hardening
    - TASK-077-treat-terminal-non-success-run-outcomes-as-cli-failures
    - TASK-085-harden-run-finalization-and-orphaned-run-recovery
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-15T16:33:57Z"
areas:
    - runner
    - container
    - evidence
    - cli
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: The fix is a branchy state transition in `writeTerminalStatus` that should be pinned with deterministic unit coverage.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Container cleanup errors must be simulated against a real `Process.Cleanup` to prove the reconciled outcome holds.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: End-to-end coverage is nice to have, but the core contract can be exercised from integration level.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Terminal-state branching is easy to weaken to a shape that still passes the happy path.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check should confirm that a cleanup failure on a successful run produces a consistent CLI exit, evidence state, and promote eligibility.
---

## Summary

Make the runner's terminal state and the CLI's exit code agree even when `Process.Cleanup` fails. `Runner.writeTerminalStatus` today writes `status.json` first and only then calls `Process.Cleanup`. If `docker rm -f` fails on an otherwise successful run, the function returns a non-`TerminalStateError`, so `cmd/tessariq/run.go` reports a generic failure and exits non-zero — while `status.json` still records `success` and the worktree is cleaned up. A subsequent `tessariq promote last-0` then happily promotes the run the CLI just told the user had failed.

## Supersedes

- BUG-059 from `planning/BUGS.md`.

## Acceptance Criteria

- A successful run whose container cleanup fails must end in a consistent state: either the run is recorded as success *and* the CLI exits zero (with the cleanup error surfaced as a warning), or the run is recorded as non-success *and* the CLI exits non-zero and `status.json` agrees.
- The implementation picks one of the two strategies and documents the choice in code and the task completion note. The preferred strategy is "cleanup before final status write, downgrade on failure"; the alternative is "keep order, treat post-write cleanup error as non-fatal warning". Mixed behavior is not acceptable.
- Whatever state is recorded, the CLI exit code, the printed output, the `status.json` contents, and the index entry must all agree on success vs. failure.
- Idempotent cleanup (per TASK-085 / BUG-028) is preserved: a second `docker rm -f` attempt on the same container does not cause spurious errors.
- `promote` cannot promote a run that the CLI reported as failed due to cleanup error; the terminal state on disk must match what the operator saw.
- Worktree cleanup discipline remains correct: a genuinely-successful run that was only downgraded due to cleanup failure should either keep the worktree for post-mortem (matching the non-success cleanup rule) or document why it still removes it.

## Test Expectations

- Start with a failing unit test that injects a `Process.Cleanup` error on a successful run and asserts the reconciled outcome (consistent status/exit/printed output/index entry).
- Add unit coverage proving that a container cleanup error on a non-success run never escalates to a double-downgrade or masks the original terminal state.
- Add integration coverage using a real `container.Process` (or a test double that closely mirrors `docker rm -f` failure) to pin the end-to-end state.
- Add a promote-level test that shows a run reported as failed due to cleanup error is not silently promotable.
- Run mutation testing because the state-transition logic is the single source of truth for terminal-state finalization.
- Manual test: run an agent that exits 0, inject a `docker rm` failure (e.g., by racing a second `docker rm -f` from another terminal), and verify the CLI exit, `status.json`, and `tessariq promote` all agree.

## TDD Plan

1. RED: add a unit test where `Process.Cleanup` returns an error on a successful run and assert the reconciled outcome.
2. RED: add the symmetric test for a failed run where cleanup also fails.
3. GREEN: invert the ordering inside `writeTerminalStatus` (or switch to the warning strategy) and make the tests pass.
4. GREEN: update `cmd/tessariq/run.go` so the CLI output path matches the new contract.
5. REFACTOR: collapse any duplicate state-update paths into a single helper so the invariant is obvious.
6. VERIFY: rerun the automated ladder plus the manual cleanup-failure scenario.

## Notes

- This is a state-consistency bug, not a security bug; severity is low but the fix is still required for v0.1.0 because promote correctness depends on it.
- Do not introduce a new terminal state name — reuse `StateFailed` or the existing `StateSuccess` with a warning, depending on strategy.
- Keep the change scoped to `writeTerminalStatus` and its direct CLI consumer. Do not refactor the broader lifecycle ownership contract in the same commit.
- Coordinate with TASK-085's finalization discipline so the fix does not re-open the reconciliation path for stale `running` entries.
- 2026-04-15T16:33:57Z: Reordered container cleanup to run before the terminal status write so docker rm -f failures downgrade success→failed, stamp status.cleanup_error, and flow through TerminalStateError. Added promote.ErrCleanupFailed guard, CLI cleanup_error surface, unit + integration tests, and manual-test MT-001..MT-004. Mutation efficacy 86.26% > 70%.
