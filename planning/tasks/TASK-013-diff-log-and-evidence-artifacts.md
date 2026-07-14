---
id: TASK-013-diff-log-and-evidence-artifacts
title: Emit diff, log, agent, and runtime evidence artifacts
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#evidence-contract
dependencies:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-027-container-lifecycle-and-mount-isolation
updated_at: "2026-03-31T23:04:49Z"
---

## Summary

Finish required v0.1.0 evidence artifact emission, including diff outputs, capped logs, and final completeness checks across artifacts produced by earlier tasks.

## Acceptance Criteria

- This task closes remaining evidence gaps across artifacts already seeded by earlier tasks.
- Required artifacts are present for every run: `manifest.json`, `status.json`, `agent.json`, `runtime.json`, `task.md`, `run.log`, `runner.log`, and `workspace.json`.
- `diff.patch` and `diffstat.txt` exist only when code changes are present.
- Proxy-mode artifacts remain conditional: `egress.compiled.yaml` and `egress.events.jsonl` only in proxy mode, and optional `squid.log` only when emitted by the proxy runtime.
- Capped logs include an explicit truncation marker when trimmed.
- Required JSON artifacts preserve `schema_version: 1` and their earlier minimum-shape guarantees.

## Test Expectations

- Add unit tests for artifact-path derivation and truncation behavior.
- Add unit tests for log truncation boundary conditions: exactly at the size limit, one byte over the limit, and well under the limit.
- Add integration tests for artifact generation using Testcontainers-backed collaborators only.
- Add an integration test for holistic evidence completeness: assert all 8 required files (`manifest.json`, `status.json`, `agent.json`, `runtime.json`, `task.md`, `run.log`, `runner.log`, `workspace.json`) exist, are non-empty, and JSON artifacts parse with valid `schema_version: 1`.
- Add a thin e2e evidence-presence flow for the run pipeline.
- Run mutation testing because conditional artifact logic is easy to weaken accidentally.

## TDD Plan

- Start with a failing unit test for log truncation markers and a failing integration test for diff artifact generation.

## Notes

- Required JSON artifacts must keep `schema_version: 1`.
- Proxy artifact production is owned by `TASK-012`; this task enforces completeness, diff outputs, and capped-log behavior at the end of the run pipeline.
- 2026-03-31T23:04:49Z: Implemented diff.patch/diffstat.txt generation, log capping with truncation markers, evidence completeness check. All unit tests pass. Manual test completed with local-only artifacts. Verification completed with 0 findings and local-only artifacts.
