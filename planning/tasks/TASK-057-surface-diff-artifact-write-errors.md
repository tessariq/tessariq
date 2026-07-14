---
id: TASK-057-surface-diff-artifact-write-errors
title: Surface WriteDiffArtifacts errors as warnings instead of silent discard
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#required-artifacts
dependencies:
    - TASK-013-diff-log-and-evidence-artifacts
updated_at: "2026-04-02T08:31:34Z"
---

## Summary

`WriteDiffArtifacts` error is silently discarded with `_` at `run.go:216`. Replace with a warning to stderr so users know when required diff evidence is missing.

## Supersedes

- BUG-024 from `planning/BUGS.md`.

## Acceptance Criteria

- When `WriteDiffArtifacts` returns an error, a warning is printed to stderr.
- The warning format matches the existing `appendIndexEntry` warning pattern (`warning: ...`).
- The run still completes successfully — diff artifact failure is a warning, not fatal.
- Successful diff writes produce no extra output (no change to the happy path).

## Test Expectations

- Add a unit test that triggers a `WriteDiffArtifacts` error and asserts the warning appears on stderr.
- Add a regression unit test confirming no warning on successful diff writes.

## TDD Plan

1. RED: add a failing test that expects a stderr warning when `WriteDiffArtifacts` fails.
2. GREEN: replace `_ =` with an error check that prints to `cmd.ErrOrStderr()`.
3. REFACTOR: ensure the warning format is consistent with `appendIndexEntry`.

## Notes

- Likely files: `cmd/tessariq/run.go:216`.
- The fix follows the same pattern already used by `appendIndexEntry` at `run.go:344-362`.
- Keep the warning non-fatal to avoid blocking runs over transient git or I/O errors.
- 2026-04-02T08:31:34Z: warnDiffArtifacts helper prints warning to stderr on error; unit tests cover both error and nil paths; mutation testing 85.5% efficacy; manual tests pass all 4 acceptance criteria
