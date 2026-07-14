---
id: TASK-036-base-sha-consistency-between-manifest-and-workspace
title: Eliminate base_sha race between manifest and workspace evidence
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#workspace-guarantees
dependencies:
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-022-agent-and-runtime-evidence-migration
updated_at: "2026-04-01T12:09:20Z"
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
