---
id: TASK-003-dirty-repo-gate-and-task-ingest
title: Enforce clean-repo gating and ingest task metadata
status: todo
priority: p0
depends_on:
  - TASK-002-run-cli-flags-and-manifest-bootstrap
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
  - specs/tessariq-v0.1.0.md#repository-model
  - specs/tessariq-v0.1.0.md#user-authored-inputs
  - specs/tessariq-v0.1.0.md#workspace-guarantees
  - specs/tessariq-v0.1.0.md#acceptance-scenarios
  - specs/tessariq-v0.1.0.md#failure-ux
updated_at: 2026-03-29T12:06:20Z
areas:
  - git
  - evidence
verification:
  unit:
    required: true
    commands:
      - go test ./...
    rationale: Parsing H1 titles, basename fallback, and dirty-repo classification should start with unit tests.
  integration:
    required: false
    commands:
      - go test -tags=integration ./...
    rationale: Add containerized integration coverage only if git command orchestration needs real-process validation.
  e2e:
    required: false
    commands:
      - go test -tags=e2e ./...
    rationale: No full user-flow assertion is needed until run execution is complete.
  mutation:
    required: true
    commands:
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Dirty-repo gating and metadata fallback logic are good mutation-testing targets.
---

## Summary

Fail early on dirty repositories and copy the task file into evidence with stable title extraction.

## Acceptance Criteria

- Dirty repositories fail before any container work starts when the repo has staged, unstaged, or untracked non-ignored files.
- The task file is copied exactly to evidence as `task.md`.
- `task_title` is derived from the first Markdown H1 when present, or the task-file basename without extension when no H1 exists.
- The derived `task_title` is written into the manifest before runner bootstrap begins.
- Dirty-repo failure messaging tells the user to commit, stash, or clean the repository first.

## Test Expectations

- Add unit tests for title extraction and dirty-repo gate classification.
- Integration tests are deferred unless git process behavior needs containerized validation.
- E2E tests are deferred until run orchestration exists end to end.
- Run mutation testing because branchy decision logic is involved.

## TDD Plan

- Start with a failing unit test for the dirty-repo preflight decision and task title extraction.

## Notes

- Task-path validation is intentionally owned by `TASK-002`; this task stays focused on dirty-repo gating and task ingestion.
