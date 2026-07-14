---
id: TASK-073-print-evidence-path-on-run-failure
title: Print failed run evidence details before returning run errors
status: completed
priority: low
spec_ref: specs/tessariq-v0.1.0.md#tessariq-run-task-path
dependencies:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-006-tmux-session-and-detached-attach-guidance
updated_at: "2026-04-05T10:25:11Z"
---

## Summary

When a run fails after evidence bootstrap, the CLI currently returns only the error string. Print stable failure details such as `run_id` and `evidence_path` so users can immediately inspect the failed run's artifacts.

## Supersedes

- BUG-039 from `planning/BUGS.md`.

## Acceptance Criteria

- Failed runs that already have an evidence directory print at least `run_id` and `evidence_path` before returning a non-zero error.
- Success-path output remains unchanged.
- Pre-bootstrap failures that do not yet have evidence continue to fail without printing bogus success-formatted fields.
- Output remains script-friendly and clearly separated from the returned error message.

## Test Expectations

- Add unit tests for failure-path output when evidence exists and when it does not.
- Add an e2e regression that forces a run failure after bootstrap and asserts the printed evidence path.
- Add regression coverage that successful runs still print the same detached guidance fields.

## TDD Plan

1. RED: add a failing test for a post-bootstrap run failure that expects `evidence_path` in CLI output.
2. GREEN: print failure details before returning the run error.
3. GREEN: keep preflight and success output contracts unchanged.

## Notes

- Likely files: `cmd/tessariq/run.go` and related command tests.
- Prefer reusing the existing output formatting helpers instead of inventing a separate failure-only format unless the success fields would be misleading.
- 2026-04-05T10:25:11Z: Post-bootstrap failures now print run_id and evidence_path via named-return defer; 2 unit tests, 1 new e2e test, 2 updated e2e tests; 4/4 manual tests pass
