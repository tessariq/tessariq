---
id: TASK-051-attach-repo-local-evidence-path-validation
title: Reject non-repo evidence paths during attach resolution
status: done
priority: p0
depends_on:
    - TASK-007-attach-command-live-run-resolution
    - TASK-014-run-index-and-run-ref-resolution
    - TASK-045-validate-index-entry-shape-before-resolution
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#core-workflow
    - specs/tessariq-v0.1.0.md#generated-runtime-state
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-02T10:31:36Z"
areas:
    - attach
    - evidence
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Evidence-path validation is deterministic and should be exercised at unit level.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Attach liveness checks should be verified against forged index data and real session checks.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: This is a user-visible trust boundary on the attach command.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Path-validation branches should not silently weaken.
    manual_test:
        required: true
        commands: []
        rationale: Confirms attach cannot validate liveness from external evidence outside the repository.
---

## Summary

`attach` currently trusts `index.jsonl` `evidence_path` values and will read `status.json` from arbitrary host paths. Require repo-local evidence under `<repo>/.tessariq/runs/<run_id>` before any liveness checks.

## Supersedes

- BUG-016 from `planning/BUGS.md`.

## Acceptance Criteria

- `attach` rejects absolute `evidence_path` values from the index.
- Relative `evidence_path` values are cleaned and rejected if they escape the repository root or `.tessariq/runs/` subtree.
- `attach` fails before reading `status.json` when the evidence path is not repo-local for the resolved run.
- Failure output remains attach-appropriate and does not allow external evidence to authorize a live attach.

## Test Expectations

- Add unit tests for absolute-path rejection, traversal rejection, and canonical repo-local acceptance.
- Add integration or e2e adversarial coverage with a forged external evidence directory and assert attach refuses it.
- Add regression coverage that valid repo-local live runs still attach normally.

## TDD Plan

1. RED: add a failing test for an external absolute `evidence_path`.
2. GREEN: normalize and validate evidence directories before reading status.
3. REFACTOR: keep attach liveness checks operating only on trusted evidence paths.
4. GREEN: verify external evidence can no longer satisfy the live-run gate.

## Notes

- Likely files: `internal/attach/attach.go` and attach integration/e2e tests.
- Prefer reusing the same repo-local evidence validation rules as promote hardening where practical.
- 2026-04-02T10:31:36Z: Implemented repo-local attach evidence-path validation before liveness checks; verified with go test ./..., go test -tags=integration ./..., go test -tags=e2e ./..., gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70 (85.63% efficacy), and manual test artifacts under planning/artifacts/manual-test/TASK-051-attach-repo-local-evidence-path-validation/20260402T102743Z.
