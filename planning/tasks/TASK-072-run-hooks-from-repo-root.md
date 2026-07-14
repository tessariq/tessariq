---
id: TASK-072-run-hooks-from-repo-root
title: Run pre and verify hooks from the repository root
status: completed
priority: low
spec_ref: specs/tessariq-v0.1.0.md#tessariq-run-task-path
dependencies:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
updated_at: "2026-04-05T10:17:14Z"
---

## Summary

Pre and verify hooks currently execute from the evidence directory, which breaks relative project commands. Pass the repository root through the runner so hooks run in the expected project context.

## Supersedes

- BUG-038 from `planning/BUGS.md`.

## Acceptance Criteria

- `--pre` hooks execute with CWD set to the repository root.
- `--verify` hooks execute with CWD set to the repository root.
- Relative-path project commands such as `ls Makefile`, `go test ./...`, or `pytest tests/` work without requiring an inline `cd` prefix.
- Existing hook ordering, logging, and failure propagation remain unchanged.

## Test Expectations

- Add unit tests that prove the runner passes the repository root into both pre and verify hook execution.
- Add integration coverage that exercises a relative-path hook command against a real temporary repository layout.
- Add regression coverage that failed hooks still write the same runner status and log output.

## TDD Plan

1. RED: add a failing test that shows hooks currently execute from `.tessariq/runs/<run_id>`.
2. GREEN: thread repository-root workdir through the runner and hook calls.
3. GREEN: rerun hook failure-path coverage to confirm behavior is unchanged apart from CWD.

## Notes

- Likely files: `cmd/tessariq/run.go`, `internal/runner/runner.go`, and hook tests.
- Keep the fix minimal; the problem is the chosen workdir, not hook ordering or shell behavior.
- 2026-04-05T10:17:14Z: Added RepoRoot field to Runner struct; pre/verify hooks now execute from repository root instead of evidence directory. Unit and integration tests added. Manual test confirms relative-path commands work.
