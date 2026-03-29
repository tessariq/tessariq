---
id: TASK-016-v0-1-0-spec-conformity-verification
title: Harden tracked-work validation and active-spec verification gates
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
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#tessariq-attach-run-ref
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-29T12:06:20Z"
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
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: This task hardens verification tooling rather than introducing a new runtime user flow.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Verification logic and acceptance-scenario mapping should hold the mutation threshold too.
    manual_test:
        required: true
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
---

## Summary

Harden `validate-state` and `verify --profile spec` so broken task/spec links, stale anchors, and missing active-spec ownership of normative contracts fail loudly in normal workflow use and CI.

## Acceptance Criteria

- `go run ./cmd/tessariq-workflow validate-state` fails when a task points at a missing spec file or dead heading anchor.
- `go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json` reports scope metadata for the active milestone spec and emits high-severity findings for uncovered normative contracts, acceptance scenarios, failure-UX rows, or evidence-compatibility rules in that active spec.
- Workflow validation fixtures cover both the stale-link regression and a missing-coverage regression that previously passed silently.
- Task and CI documentation explain that the validation gates are hard failures, not advisory output.

## Test Expectations

- Add unit tests for spec-reference resolution, dead-anchor detection, active-scope reporting, and coverage mapping for normative contracts, acceptance scenarios, failure rows, and evidence rules.
- Integration tests are optional unless the verifier grows real collaborator dependencies.
- E2E tests are not required for this task because it hardens verification tooling rather than introducing a new runtime user flow.
- Run mutation testing because verification logic is easy to weaken accidentally.

## TDD Plan

- Start with a failing unit test for a dead spec anchor and a failing verifier expectation for active-scope reporting.

## Notes

- This task makes the planning/spec validation gate trustworthy before the final v0.1.0 closeout sweep.
