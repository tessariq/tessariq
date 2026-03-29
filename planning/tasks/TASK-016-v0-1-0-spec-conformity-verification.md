---
id: TASK-016-v0-1-0-spec-conformity-verification
title: Verify v0.1.0 implementation conformity against the spec
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
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
  - specs/tessariq-v0.1.0.md#product-intent
  - specs/tessariq-v0.1.0.md#cli-run
  - specs/tessariq-v0.1.0.md#cli-attach
  - specs/tessariq-v0.1.0.md#cli-promote
  - specs/tessariq-v0.1.0.md#evidence-contract
  - specs/tessariq-v0.1.0.md#acceptance-init-skeleton
  - specs/tessariq-v0.1.0.md#acceptance-run-clean-repo
  - specs/tessariq-v0.1.0.md#acceptance-run-dirty-repo
  - specs/tessariq-v0.1.0.md#acceptance-attach-live-run
  - specs/tessariq-v0.1.0.md#acceptance-promote-one-commit
  - specs/tessariq-v0.1.0.md#acceptance-promote-zero-diff
  - specs/tessariq-v0.1.0.md#acceptance-missing-evidence
  - specs/tessariq-v0.1.0.md#acceptance-proxy-allowlists
updated_at: 2026-03-29T00:00:00Z
areas:
  - verification
  - spec
verification:
  unit:
    required: true
    commands:
      - go test ./...
    rationale: Add unit tests for spec-coverage and task-coverage verification helpers.
  integration:
    required: false
    commands:
      - go test -tags=integration ./...
    rationale: Integration coverage is optional unless the conformity verifier starts real collaborators.
  e2e:
    required: true
    commands:
      - go test -tags=e2e ./...
    rationale: Final conformity should include thin end-to-end coverage of the critical user journeys.
  mutation:
    required: true
    commands:
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Verification logic and acceptance-scenario mapping should hold the mutation threshold too.
---

## Summary

Run the final v0.1.0 conformity sweep against the normative spec and create follow-up tasks for any unresolved gaps.

## Acceptance Criteria

- `go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json` passes with no unresolved high-severity findings.
- Every normative contract and acceptance scenario is covered by tasks and implemented behavior.
- Follow-up tasks are created for any remaining medium-or-higher gaps.

## Test Expectations

- Add unit tests for spec-reference coverage and acceptance-scenario mapping.
- Integration tests are optional unless the verifier grows real collaborator dependencies.
- Add thin end-to-end coverage for the critical user-visible workflows before calling the milestone done.
- Run mutation testing because verification logic is easy to weaken accidentally.

## TDD Plan

- Start with a failing unit test for missing spec coverage and a failing spec-verifier report expectation.

## Notes

- This task is the required final gate before considering `v0.1.0` complete.
