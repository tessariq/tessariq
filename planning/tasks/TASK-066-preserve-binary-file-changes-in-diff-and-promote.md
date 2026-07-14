---
id: TASK-066-preserve-binary-file-changes-in-diff-and-promote
title: Preserve binary file changes in diff artifacts and promote
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#evidence-contract
dependencies:
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-049-promote-require-diffstat-for-changed-runs
updated_at: "2026-04-03T08:54:42Z"
---

## Summary

`git diff` is currently emitted without `--binary`, so binary changes are reduced to text-only placeholders and disappear during `git apply` on promote. Emit binary-capable patch artifacts so promote preserves the same file set the agent produced.

## Supersedes

- BUG-033 from `planning/BUGS.md`.

## Acceptance Criteria

- `diff.patch` includes binary hunks when the worktree contains binary file changes.
- `tessariq promote <run-ref>` applies those binary hunks successfully.
- Runs with only text changes keep their current behavior.
- Promotion does not silently drop binary additions or modifications.

## Test Expectations

- Add integration coverage creating a binary change and asserting `diff.patch` contains binary patch data.
- Add promote integration or e2e coverage proving the promoted branch contains the binary file bytes.
- Add regression coverage for ordinary text-only diffs.

## TDD Plan

1. RED: add a failing binary-file diff regression.
2. GREEN: generate binary-capable patches from `git diff`.
3. GREEN: verify promote applies the resulting patch without data loss.

## Notes

- Likely files: `internal/git/diff.go`, git diff integration tests, and promote integration/e2e tests.
- Preserve existing diffstat behavior unless binary coverage shows a related gap there too.
- 2026-04-03T08:54:42Z: Added --binary to git diff command; binary hunks now survive run-to-promote flow. Integration tests for binary diff and binary promote round-trip added. Manual test 3/3 pass. Mutation efficacy 92.33%.
