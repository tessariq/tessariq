---
id: TASK-054-reject-symlinked-external-task-paths
title: Reject symlinked task files whose real target escapes the repository
status: done
priority: p0
depends_on:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-003-dirty-repo-gate-and-task-ingest
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#repository-model
    - specs/tessariq-v0.1.0.md#user-authored-inputs
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-02T07:40:35Z"
areas:
    - cli
    - evidence
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Real-path validation logic should start with focused path-handling unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Symlink behavior is filesystem-dependent and should be verified with real repo fixtures.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: This is a repository-boundary contract on the user-visible run command.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Boundary checks are safety-critical and branch-heavy.
    manual_test:
        required: true
        commands: []
        rationale: Confirms repo-local symlinks cannot smuggle external task content into evidence.
---

## Summary

Task-path validation currently checks only the lexical joined path, then follows symlinks during `os.Stat`. Resolve the real filesystem target before acceptance so repo-local symlinks cannot point at external Markdown files.

## Supersedes

- BUG-022 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq run <task-path>` rejects a symlink whose real target is outside the current repository.
- Failure happens before task read, evidence bootstrap side effects, or container start.
- Ordinary in-repo Markdown files still pass unchanged.
- Symlinks whose resolved target remains inside the repository are either accepted intentionally or rejected consistently, but the repository-boundary contract is enforced on the real target path.
- `task.md` evidence always comes from a real in-repo task file target.

## Test Expectations

- Add unit tests for real-path escape detection using symlink fixtures.
- Add integration or e2e coverage with a repo-local symlink to an external Markdown file and assert the run fails as an invalid outside-repo task path.
- Add regression coverage for normal in-repo Markdown files and, if supported, in-repo symlink targets.

## TDD Plan

1. RED: add a failing test with a repo-local symlink to an external Markdown file.
2. GREEN: resolve symlinks before enforcing repository-boundary checks.
3. REFACTOR: keep task-path logic explicit about lexical validation vs real-target validation.
4. GREEN: verify task-copy and manifest paths only proceed for accepted in-repo targets.

## Notes

- Likely files: `internal/run/taskpath.go`, `cmd/tessariq/run.go`, `internal/run/taskcopy.go`, and task-path tests.
- Keep user-facing failure text aligned with the existing invalid-task-path guidance.
- 2026-04-02T07:40:35Z: Added filepath.EvalSymlinks boundary re-check in ValidateTaskPath. Unit tests: symlink-outside-repo, symlink-inside-repo, broken-symlink. E2e test: TestE2E_SymlinkToExternalTaskRejected. Manual tests: 5/5 pass. Mutation efficacy: 85.19%. Verification: 0 findings.
