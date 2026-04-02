---
id: TASK-066-preserve-binary-file-changes-in-diff-and-promote
title: Preserve binary file changes in diff artifacts and promote
status: todo
priority: p0
depends_on:
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-049-promote-require-diffstat-for-changed-runs
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
updated_at: "2026-04-02T14:59:17Z"
areas:
    - git
    - evidence
    - promote
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Diff-command construction and promote-path handling should be covered at unit level first.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Real git repositories are needed to verify binary patches survive run-to-promote flow.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: This is user-visible data-preservation behavior in the primary workflow.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Diff generation is safety-critical because silent data loss is otherwise easy to miss.
    manual_test:
        required: true
        commands: []
        rationale: Confirms a run that changes a binary file promotes those bytes correctly.
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
