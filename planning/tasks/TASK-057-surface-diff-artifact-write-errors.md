---
id: TASK-057-surface-diff-artifact-write-errors
title: Surface WriteDiffArtifacts errors as warnings instead of silent discard
status: done
priority: p1
depends_on:
    - TASK-013-diff-log-and-evidence-artifacts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#required-artifacts
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-02T08:31:34Z"
areas:
    - evidence
    - cli
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: The warning branch is deterministic and should be covered by unit tests.
    integration:
        required: false
        commands: []
        rationale: The fix is a warning-print branch with no new collaborator boundary.
    e2e:
        required: false
        commands: []
        rationale: Unit coverage is sufficient for a stderr warning branch.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: The new error-handling branch is safety-critical for evidence completeness.
    manual_test:
        required: true
        commands: []
        rationale: Confirms the warning appears when diff generation fails and is absent on success.
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
