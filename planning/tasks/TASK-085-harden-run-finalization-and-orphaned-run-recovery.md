---
id: TASK-085-harden-run-finalization-and-orphaned-run-recovery
title: Harden run finalization and orphaned run recovery across all supported agents
status: done
priority: p0
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-014-run-index-and-run-ref-resolution
    - TASK-024-claude-code-agent-runtime-integration
    - TASK-025-opencode-agent-runtime-integration
    - TASK-027-container-lifecycle-and-mount-isolation
    - TASK-028-container-session-streaming-and-cleanup-hardening
    - TASK-071-implement-run-attach-live-session
    - TASK-077-treat-terminal-non-success-run-outcomes-as-cli-failures
    - TASK-080-opencode-interactive-support
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#core-workflow
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#tessariq-attach-run-ref
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#required-artifacts
    - specs/tessariq-v0.1.0.md#failure-ux
    - specs/tessariq-v0.1.0.md#success-metrics
updated_at: "2026-04-13T19:37:30Z"
areas:
    - runner
    - lifecycle
    - evidence
    - attach
    - adapters
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: The fix will change signal handling, terminal-state persistence, and stale-run reconciliation logic that should be covered deterministically at the unit level.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The bug sits at the process and container lifecycle boundary, so integration coverage should exercise real Docker-backed runner behavior without relying only on mocks.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: User-visible correctness depends on full CLI runs emitting terminal status and usable run resolution for every supported agent.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Terminal-state and recovery code is branchy and easy to weaken with superficial fixes; mutation testing should guard the core decision logic.
    manual_test:
        required: true
        commands: []
        rationale: This bug involves real process interruption and orphan recovery, so a built CLI manual check is required in addition to automated tests.
---

## Summary

Fix the run lifecycle so `tessariq` does not leave `status.json`, `index.jsonl`, containers, or worktrees in a permanently `running` state when the host-side supervisor is interrupted or disappears after the agent container has already started.

The fix must work for every supported agent, not just OpenCode. Detached and interactive flows for both `claude-code` and `opencode` must either finalize cleanly or be reconciled into a non-running terminal state that matches the actual lifecycle outcome.

## Acceptance Criteria

- A normal detached run reaches a terminal `status.json` state and appends a terminal `index.jsonl` entry for both supported agents: `claude-code` and `opencode`.
- A run must not remain indefinitely `running` after the agent container has already exited, regardless of whether the exit happened before or after the host-side `tessariq` process was interrupted.
- If `tessariq run` receives an interrupt or termination signal after the container has started, the implementation records an appropriate non-running terminal outcome and performs best-effort cleanup rather than leaving only the initial `running` status behind.
- If the host-side supervisor disappears before it can write the final terminal status, subsequent lifecycle resolution must reconcile the stale run so attach and other run-resolution paths do not continue treating it as actively running forever.
- Reconciliation must be agent-agnostic: it must work for all supported agents and must not depend on agent-specific log formats or tool events.
- Reconciled or interrupted runs must preserve valid evidence artifacts; `status.json` and `index.jsonl` must agree on the final non-running state.
- Container cleanup semantics remain correct: successful and reconciled runs must not leave user-confusing stale live containers behind, and failure paths must not regress the existing cleanup guarantees.
- Worktree cleanup semantics remain correct for failed or interrupted finalization paths, with any intentional retained-worktree behavior documented and covered by tests.
- `tessariq attach` and any run-ref resolution that relies on the run index must not prefer a stale `running` entry when the run is already terminal or orphaned.
- The regression described in `../git-test` is covered by automated tests that prove the repository cannot be left with only the initial `running` state after the agent completed useful work.
- `TASK-017-v0-1-0-spec-conformity-closeout` depends on this task so v0.1.0 closeout cannot run before the stale-run lifecycle bug is fixed.

## Test Expectations

- Start with the smallest failing unit tests around terminal-status persistence and any new reconciliation helper or signal-aware finalization path.
- Add or update integration tests in the runner and container lifecycle layers to cover: container exits after useful work, supervisor interruption, and best-effort cleanup behavior.
- Add targeted e2e coverage for detached `claude-code` and detached `opencode` runs asserting terminal `status.json`, terminal `index.jsonl`, and no stale active run remains.
- Add at least one interruption-path e2e or integration test that simulates the host-side CLI being terminated after container start and verifies the run is later reconciled into a non-running state.
- Add coverage for run-ref or attach resolution so stale `running` entries do not win once reconciliation is possible.
- Use Testcontainers helpers for any integration or e2e collaborator needs; do not introduce host-tool-dependent local test scaffolding.
- Run the full automated verification ladder from front matter because this fix crosses lifecycle orchestration, evidence, and CLI behavior.
- Run manual testing against a built CLI and capture evidence that both supported agents finalize correctly and that an interrupted run no longer stays `running` forever.

## TDD Plan

1. RED: add a failing unit or integration test that reproduces the stale `running` evidence path when the supervisor is interrupted after process start.
2. RED: add failing CLI-level coverage showing detached runs for both supported agents must end with terminal status and terminal run-index state.
3. GREEN: implement the smallest robust finalization and/or reconciliation path so terminal state is persisted even across supervisor interruption scenarios.
4. GREEN: update attach or run-ref resolution paths if needed so orphaned runs are reconciled before they are treated as live.
5. REFACTOR: simplify lifecycle ownership boundaries so status writing, index updates, and cleanup all happen from one clearly testable path.
6. VERIFY: run required automated suites, manual testing, and workflow validation before marking the task done.

## Notes

- Investigation in `../git-test` found multiple OpenCode runs where the agent container completed useful work and exited `0`, but the repository was left with only the initial `running` `status.json` and `index.jsonl` entry.
- The same fix must apply to all supported agents, because the underlying failure is host-side lifecycle finalization rather than agent-specific task execution.
- Likely touched areas include `cmd/tessariq/run.go`, `internal/runner/`, `internal/container/`, attach or run-index resolution paths, and the relevant integration and e2e suites.
- The preferred fix is one that is robust to host-process interruption, not a narrow OpenCode-only patch.
- 2026-04-13T19:37:30Z: Implemented signal-aware terminal finalization, deferred container cleanup, and stale-run reconciliation for attach/promote. Evidence: planning/artifacts/manual-test/TASK-085-harden-run-finalization-and-orphaned-run-recovery/20260413T193600Z/report.md ; planning/artifacts/verify/task/TASK-085-harden-run-finalization-and-orphaned-run-recovery/20260413T193559Z/report.json ; go test ./... ; go vet ./... ; go test -tags=integration ./... ; go test -tags=e2e ./... ; gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
