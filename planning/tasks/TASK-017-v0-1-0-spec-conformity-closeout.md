---
id: TASK-017-v0-1-0-spec-conformity-closeout
title: Run the final v0.1.0 spec conformity closeout sweep
status: todo
priority: p0
depends_on:
    - TASK-001-init-skeleton-and-gitignore
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-003-dirty-repo-gate-and-task-ingest
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-006-tmux-session-and-detached-attach-guidance
    - TASK-007-attach-command-live-run-resolution
    - TASK-008-adapter-contract-and-adapter-json
    - TASK-009-claude-code-adapter
    - TASK-010-opencode-adapter
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-014-run-index-and-run-ref-resolution
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-016-v0-1-0-spec-conformity-verification
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#release-intent
    - specs/tessariq-v0.1.0.md#product-intent
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#tessariq-attach-run-ref
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
    - specs/tessariq-v0.1.0.md#success-metrics
updated_at: "2026-03-29T12:06:20Z"
areas:
    - verification
    - spec
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Keep spec-coverage and acceptance-scenario mapping checks under unit-test control during the closeout sweep.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: Integration coverage is optional unless the closeout sweep adds real collaborator dependencies.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Final conformity still requires thin end-to-end evidence for the critical user-visible workflows.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Closeout relies on verification logic that should still meet the mutation threshold.
    manual_test:
        required: true
        commands: []
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
---

## Summary

Run the final v0.1.0 conformity sweep against the normative spec after the strengthened validation tooling is in place.

## Acceptance Criteria

- `go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json` passes with no unresolved high-severity findings.
- Every normative contract, acceptance scenario, failure-UX row, and evidence-compatibility rule in the active v0.1.0 spec is covered by tasks and implemented behavior.
- The closeout explicitly records each v0.1.0 success metric as met, not yet measurable, or follow-up required; it must not silently ignore the section.
- Regenerated verification artifacts and `planning/STATE.md` validation metadata point at the final passing sweep.

## Test Expectations

- Add unit tests only if closeout uncovers a missing verifier assertion.
- Integration tests are optional unless the closeout workflow grows real collaborator dependencies.
- Add a full-pipeline e2e test covering the primary user journey end-to-end: `init -> create task -> run (detached) -> wait for completion -> promote -> verify branch, commit, trailers, and evidence artifacts`. This is the single most important e2e test for v0.1.0.
- Add error-path e2e tests for the two most common user-facing failures not covered by earlier tasks: dirty-repo rejection (spec failure-UX row) and invalid-task-path rejection (spec failure-UX row).
- Add thin end-to-end coverage for any other still-uncovered critical user-visible flow before marking the milestone done.
- Run mutation testing because the final gate should not rely on brittle or weakened verification logic.

## TDD Plan

- Start with the failing spec closeout sweep, then fix the highest-signal remaining gap before rerunning the gate.

## Notes

- This is the required final gate before considering `v0.1.0` complete.
