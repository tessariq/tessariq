---
id: TASK-052-attach-run-id-evidence-path-consistency
title: Require attach evidence to match the resolved run_id
status: todo
priority: p1
depends_on:
    - TASK-007-attach-command-live-run-resolution
    - TASK-051-attach-repo-local-evidence-path-validation
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#core-workflow
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-01T20:03:47Z"
areas:
    - attach
    - evidence
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Run-ID-to-evidence consistency checks are deterministic and unit-testable.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Attach behavior must be validated against forged mixed-run index entries and real tmux sessions.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: This is a user-visible attach integrity rule.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Identity checks are branch-heavy and easy to weaken accidentally.
    manual_test:
        required: true
        commands: []
        rationale: Confirms another run's `status.json` cannot authorize attach for the requested run.
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
