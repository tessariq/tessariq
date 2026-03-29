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
  - specs/tessariq-v0.1.0.md#acceptance-run-clean-repo
updated_at: 2026-03-29T00:00:00Z
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
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Conditional artifact emission and truncation logic benefit from mutation testing.
---

## Summary

Emit the required v0.1.0 evidence artifacts, including logs, workspace metadata, and diffs.

## Acceptance Criteria

- Required JSON and Markdown/log artifacts are present for every run.
- `diff.patch` and `diffstat.txt` exist when changes are present.
- Capped logs include an explicit truncation marker when trimmed.

## Test Expectations

- Add unit tests for artifact-path derivation and truncation behavior.
- Add integration tests for artifact generation using Testcontainers-backed collaborators only.
- Add a thin e2e evidence-presence flow for the run pipeline.
- Run mutation testing because conditional artifact logic is easy to weaken accidentally.

## TDD Plan

- Start with a failing unit test for log truncation markers and a failing integration test for diff artifact generation.

## Notes

- Required JSON artifacts must keep `schema_version: 1`.
