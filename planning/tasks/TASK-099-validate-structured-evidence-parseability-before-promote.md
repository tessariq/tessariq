---
id: TASK-099-validate-structured-evidence-parseability-before-promote
title: Reject malformed structured evidence before promote
status: done
priority: p1
depends_on:
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-088-require-proxy-evidence-completeness-before-promote
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#compatibility-rules
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#failure-ux
    - specs/tessariq-v0.1.0.md#success-metrics
updated_at: "2026-04-24T08:46:25Z"
areas:
    - promote
    - evidence
    - reliability
    - proxy
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Artifact parsing and minimum-shape validation are deterministic and should be pinned with focused unit coverage.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Promote must reject malformed non-empty evidence in a real repository before any git side effects occur.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Evidence parseability is part of the user-visible promote contract and should hold on the real CLI path.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Parse-and-shape validation branches are easy to weaken accidentally while keeping existence-only tests green.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check should show that non-empty malformed evidence is rejected with intact-evidence guidance before branch creation.
---

## Summary

`runner.CheckEvidenceCompleteness` currently treats structured artifacts as complete when they merely exist and are non-empty. That allows malformed or schema-incomplete `agent.json`, `runtime.json`, `workspace.json`, proxy artifacts, and even weakly-shaped terminal `status.json` to pass the promote gate. Strengthen evidence validation so `promote` requires required structured artifacts to be parseable and to satisfy their minimum shape.

## Supersedes

- BUG-063 from `planning/BUGS.md`.

## Acceptance Criteria

- Completeness validation parses every structured artifact required for the run shape:
  - `manifest.json`
  - `status.json`
  - `agent.json`
  - `runtime.json`
  - `workspace.json`
  - `egress.compiled.yaml` in proxy mode
  - `egress.events.jsonl` in proxy mode
- Required artifacts must have the required syntax, `schema_version`, and minimum required fields before promote can proceed.
- `promote` fails cleanly with evidence-intact guidance when a required structured artifact is malformed or missing required fields, before any git side effects.
- Honest runs with valid evidence remain promotable.
- The implementation keeps one promote-facing evidence validator rather than scattering ad-hoc per-artifact checks.

## Test Expectations

- Add unit tests for malformed-but-non-empty JSON/YAML/JSONL artifacts that currently pass completeness.
- Add unit tests for missing required fields in minimum artifact shapes.
- Add promote integration tests showing that malformed non-empty structured evidence is rejected before branch creation.
- Replace or tighten existing tests that currently encode the weak contract of schema-only stubs being sufficient evidence.
- Run mutation testing because structured-evidence validators are branch-heavy and contract-sensitive.

## TDD Plan

1. RED: add failing completeness and promote tests for malformed non-empty `agent.json`, `runtime.json`, `workspace.json`, proxy artifacts, and weakly-shaped `status.json`.
2. GREEN: implement minimal parse-and-shape validation for each required structured artifact.
3. GREEN: keep failure messaging in the existing `required evidence is missing or incomplete` family while identifying the offending artifact.
4. VERIFY: rerun unit, integration, e2e, mutation, and manual malformed-evidence checks.

## Notes

- Keep validation minimal and spec-driven: syntax, `schema_version`, and required fields, not full semantic linting.
- Reuse existing readers where possible, but do not rely on zero-value struct unmarshalling as shape validation.
- `task.md`, `run.log`, `runner.log`, `diff.patch`, and `diffstat.txt` remain file-presence/content artifacts; this task is only about structured evidence.
- Add this task to `TASK-017-v0-1-0-spec-conformity-closeout` dependencies so closeout cannot pass while non-empty malformed evidence is still promotable.
- 2026-04-24T08:46:25Z: Structured evidence parseability validation in CheckEvidenceCompleteness. All 7 artifacts validated for syntax, schema_version=1, and minimum required fields. Unit: 11 malformed-artifact cases. Integration: 6 cases. E2E: updated+passing. Mutation: 87%. Manual: 5/5 pass.
