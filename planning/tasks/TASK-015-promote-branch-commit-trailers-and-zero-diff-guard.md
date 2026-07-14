---
id: TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
title: Implement promote branch creation commit trailers and zero-diff protection
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#host-prerequisites
dependencies:
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-014-run-index-and-run-ref-resolution
updated_at: "2026-04-01T15:35:06Z"
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
- Promote accepts the new `agent.json` and `runtime.json` evidence model as the source of truth rather than the superseded `adapter.json` model.
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
- 2026-04-01T15:35:06Z: implemented tessariq promote with unit/integration/e2e coverage; manual test completed with local-only artifacts; verification completed with local-only artifacts
