---
id: TASK-072-run-hooks-from-repo-root
title: Run pre and verify hooks from the repository root
status: todo
priority: p2
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-03T12:31:03Z"
areas:
    - runner
    - hooks
    - cli
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Hook workdir selection is deterministic and should start with unit coverage.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Host-side hook execution should be exercised through real command execution, not mocks alone.
    e2e:
        required: false
        commands: []
        rationale: Focused unit and integration coverage should be sufficient for the workdir fix.
    mutation:
        required: false
        commands: []
        rationale: This is primarily a workdir-threading fix rather than complex branching.
    manual_test:
        required: true
        commands: []
        rationale: Real hook commands like `ls Makefile` or `go test ./...` should be verified from a live repo.
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
