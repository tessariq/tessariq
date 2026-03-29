---
id: TASK-013-diff-log-and-evidence-artifacts
title: Emit diff, log, and workspace evidence artifacts
status: todo
priority: p1
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-012-proxy-topology-and-egress-artifacts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: "2026-03-29T12:06:20Z"
areas:
    - evidence
    - diff
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Artifact naming, truncation markers, and conditional emission rules should be unit-tested.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Diff and log production spans real processes and should use Testcontainers-backed integration coverage only.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: A thin end-to-end check is useful because durable evidence is a core user contract.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Conditional artifact emission and truncation logic benefit from mutation testing.
---

## Summary

Finish required v0.1.0 evidence artifact emission, including diff outputs, capped logs, and final completeness checks across artifacts produced by earlier tasks.

## Acceptance Criteria

- This task closes remaining evidence gaps across artifacts already seeded by earlier tasks.
- Required artifacts are present for every run: `manifest.json`, `status.json`, `adapter.json`, `task.md`, `run.log`, `runner.log`, and `workspace.json`.
- `diff.patch` and `diffstat.txt` exist only when code changes are present.
- Proxy-mode artifacts remain conditional: `egress.compiled.yaml` and `egress.events.jsonl` only in proxy mode, and optional `squid.log` only when emitted by the proxy runtime.
- Capped logs include an explicit truncation marker when trimmed.
- Required JSON artifacts preserve `schema_version: 1` and their earlier minimum-shape guarantees.

## Test Expectations

- Add unit tests for artifact-path derivation and truncation behavior.
- Add unit tests for log truncation boundary conditions: exactly at the size limit, one byte over the limit, and well under the limit.
- Add integration tests for artifact generation using Testcontainers-backed collaborators only.
- Add an integration test for holistic evidence completeness: assert all 7 required files (`manifest.json`, `status.json`, `adapter.json`, `task.md`, `run.log`, `runner.log`, `workspace.json`) exist, are non-empty, and JSON artifacts parse with valid `schema_version: 1`.
- Add a thin e2e evidence-presence flow for the run pipeline.
- Run mutation testing because conditional artifact logic is easy to weaken accidentally.

## TDD Plan

- Start with a failing unit test for log truncation markers and a failing integration test for diff artifact generation.

## Notes

- Required JSON artifacts must keep `schema_version: 1`.
- Proxy artifact production is owned by `TASK-012`; this task enforces completeness, diff outputs, and capped-log behavior at the end of the run pipeline.
