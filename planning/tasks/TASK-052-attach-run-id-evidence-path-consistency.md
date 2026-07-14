---
id: TASK-052-attach-run-id-evidence-path-consistency
title: Require attach evidence to match the resolved run_id
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#core-workflow
dependencies:
    - TASK-007-attach-command-live-run-resolution
    - TASK-051-attach-repo-local-evidence-path-validation
updated_at: "2026-04-02T13:54:55Z"
---

## Summary

Even when `evidence_path` stays repo-local, attach currently lets one run ID borrow another run's evidence to pass the liveness check. Enforce that the evidence directory being read belongs to the same resolved `run_id`.

## Supersedes

- BUG-017 from `planning/BUGS.md`.

## Acceptance Criteria

- `attach RUN_A` fails if the resolved index entry points at `.tessariq/runs/RUN_B` or any other run's evidence directory.
- Liveness is determined only from the requested run's own evidence directory and tmux session.
- Failure messaging identifies inconsistent run evidence rather than attaching.
- Valid same-run evidence continues to work without changing normal attach UX.

## Test Expectations

- Add unit tests for directory-name / run-ID mismatch rejection.
- Add integration or e2e regression with `RUN_A` session plus `RUN_B` running evidence and assert attach refuses the mismatch.
- Add regression coverage for a valid same-run live attach.

## TDD Plan

1. RED: add a failing test where `entry.RunID` and `entry.EvidencePath` refer to different runs.
2. GREEN: validate evidence-directory identity against the resolved run ID before status/session checks.
3. REFACTOR: keep attach integrity checks adjacent to evidence-path normalization.
4. GREEN: verify only same-run evidence can satisfy liveness.

## Notes

- Likely files: `internal/attach/attach.go` and attach integration/e2e tests.
- Keep session naming sourced from the resolved run ID; the bug is that evidence liveness is not tied to it yet.
- 2026-04-02T13:54:55Z: Enforce evidence directory name matches resolved run_id during attach. Unit, integration, e2e, mutation (85.69%), and manual tests all pass.
