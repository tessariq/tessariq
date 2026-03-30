---
id: TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
title: Implement promote branch creation commit trailers and zero-diff protection
status: todo
priority: p0
depends_on:
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-014-run-index-and-run-ref-resolution
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#host-prerequisites
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-30T20:35:00Z"
areas:
    - git
    - promote
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Branch-name selection, commit-message fallback, and trailer rendering should begin with unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Promote creates real git side effects and requires Testcontainers-backed integration coverage only.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: '`run -> promote` is a primary user journey and deserves thin end-to-end coverage.'
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Guardrails around zero-diff and trailer emission should survive mutation testing.
    manual_test:
        required: true
        commands: []
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
---

## Summary

Implement `tessariq promote <run-ref>` with branch creation, exactly one commit, optional trailers, and zero-diff guards.

## Acceptance Criteria

- Promote works only for finished runs with code changes; unknown, unfinished, missing-evidence, and zero-diff cases fail without creating a branch or commit.
- The default branch name is exactly `tessariq/<run_id>`.
- The default commit message is `task_title`, with fallback `tessariq: apply run <run_id>`.
- Promote creates one branch and exactly one commit for changed runs and uses `git add -A` for the promoted delta.
- Default trailers are exactly `Tessariq-Run: <run_id>`, `Tessariq-Base: <base_sha>`, and `Tessariq-Task: <task_path>`.
- `--no-trailers` suppresses the default trailer block without changing the one-commit contract.
- Failure guidance tells the user when there were no code changes to promote or identifies the missing artifact that blocks promotion.
- Promote fails cleanly with actionable guidance when required `git` operations cannot run because `git` is missing or unavailable.

## Test Expectations

- Add unit tests for branch names, commit messages, exact trailer formatting, and `--no-trailers` behavior.
- Add unit tests for missing-`git` prerequisite handling and user guidance in promote preflight paths.
- Add unit tests for promote edge cases: branch name collision (target branch already exists), `--branch` with invalid git branch characters (`..`, spaces, `~`), promoting the same run twice (second attempt fails cleanly), commit message with unicode and special characters, `--message` with multiline string, and worktree already cleaned up before promote.
- Add unit tests verifying `git add -A` includes file deletions in the promoted delta.
- Add integration tests for promote side effects using Testcontainers-backed collaborators only.
- Add a thin e2e `run -> promote` flow for a successful changed run and a zero-diff failure path.
- Add an error-path e2e test that verifies actionable missing-`git` guidance for promote.
- Run mutation testing because guardrails and fallback logic are safety-critical.

## TDD Plan

- Start with a failing unit test for zero-diff detection and trailer rendering, then a failing e2e promote flow.

## Notes

- Use `git add -A` during promote and keep user-visible failures actionable.
