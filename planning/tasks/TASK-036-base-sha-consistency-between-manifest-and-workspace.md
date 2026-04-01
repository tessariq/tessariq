---
id: TASK-036-base-sha-consistency-between-manifest-and-workspace
title: Eliminate base_sha race between manifest and workspace evidence
status: done
priority: p1
depends_on:
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-022-agent-and-runtime-evidence-migration
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#workspace-guarantees
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-04-01T12:09:20Z"
areas:
    - workspace
    - run
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Base SHA propagation is deterministic and should be enforced with unit tests at function boundaries.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Worktree provisioning and evidence writing cross package boundaries and require integration coverage.
    e2e:
        required: false
        commands: []
        rationale: Risk is internal consistency rather than top-level CLI UX.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Data-flow regressions are subtle and mutation tests help catch missed branches.
    manual_test:
        required: false
        commands: []
        rationale: Deterministic consistency can be fully automated.
---

## Summary

`base_sha` is currently resolved in more than one place during run setup, creating a race window where `manifest.json.base_sha` and `workspace.json.base_sha` can diverge. This task centralizes base SHA capture and ensures all evidence and workspace provisioning use the same value.

## Supersedes

- BUG-004 from `planning/BUGS.md`.

## Acceptance Criteria

- `base_sha` is resolved once per run before workspace provisioning.
- `workspace.Provision` (or equivalent) consumes the caller-provided base SHA rather than re-reading repo HEAD.
- `manifest.json.base_sha` and `workspace.json.base_sha` are always identical for a given run.
- Worktree is created at the same SHA recorded in evidence.

## Test Expectations

- Add unit tests for run setup flow enforcing single-source base SHA propagation.
- Add unit tests for workspace provisioning API contract that requires explicit base SHA input.
- Add integration test asserting manifest/workspace base SHA equality under run execution.

## TDD Plan

1. RED: add failing test demonstrating mismatch potential or duplicate base SHA lookup.
2. RED: add failing test for updated `workspace.Provision` signature/contract.
3. GREEN: plumb base SHA from run command into workspace provisioning.
4. REFACTOR: remove redundant return values/unused SHA outputs if no longer needed.
5. GREEN: validate integration tests for evidence consistency.

## Notes

- Likely files: `cmd/tessariq/run.go`, `internal/workspace/provision.go`, and associated tests.
- Keep API changes minimal and localized to avoid broad churn.
- 2026-04-01T12:09:20Z: Centralized base SHA resolution: Provision now accepts caller-provided baseSHA instead of re-reading HEAD. New integration test TestProvision_Integration_UsesCallerProvidedSHA verifies the contract. Mutation efficacy 85.03%.
