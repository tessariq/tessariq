---
id: TASK-016-v0-1-0-spec-conformity-verification
title: Harden tracked-work validation and active-spec verification gates
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#product-intent
dependencies:
    - TASK-001-init-skeleton-and-gitignore
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-003-dirty-repo-gate-and-task-ingest
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-006-tmux-session-and-detached-attach-guidance
    - TASK-007-attach-command-live-run-resolution
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-014-run-index-and-run-ref-resolution
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-020-prerequisite-preflight-and-missing-dependency-ux
    - TASK-021-reference-runtime-image-and-docs
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-023-supported-agent-auth-mounts
    - TASK-024-claude-code-agent-runtime-integration
    - TASK-025-opencode-agent-runtime-integration
    - TASK-026-mount-agent-config-flag-and-config-dir-mounts
    - TASK-027-container-lifecycle-and-mount-isolation
    - TASK-033-opencode-interactive-request-recording
    - TASK-034-opencode-egress-allow-provider-bypass
    - TASK-035-init-evidence-parent-directory-permissions
    - TASK-036-base-sha-consistency-between-manifest-and-workspace
    - TASK-037-prestart-agent-binary-validation
    - TASK-038-guaranteed-worktree-cleanup-on-run-failure
    - TASK-039-capped-log-writers-and-truncation-markers
updated_at: "2026-04-01T19:05:03Z"
---

## Summary

Harden `validate-state` and `verify --profile spec` so broken task/spec links, stale anchors, and missing active-spec ownership of normative contracts fail loudly in normal workflow use and CI.

## Acceptance Criteria

- `go run ./cmd/tessariq-workflow validate-state` fails when a task points at a missing spec file or dead heading anchor.
- `go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json` reports scope metadata for the active milestone spec and emits high-severity findings for uncovered normative contracts, acceptance scenarios, failure-UX rows, host-prerequisite contracts, or evidence-compatibility rules in that active spec.
- Verification coverage explicitly understands the v0.1.0 shift from `adapter` to `agent`, the addition of `runtime.json`, and the historical compatibility alias heading kept for completed-task references.
- Workflow validation fixtures cover both the stale-link regression and a missing-coverage regression that previously passed silently.
- Task and CI documentation explain that the validation gates are hard failures, not advisory output.

## Test Expectations

- Add unit tests for spec-reference resolution, dead-anchor detection, active-scope reporting, and coverage mapping for normative contracts, acceptance scenarios, failure rows, host-prerequisite contracts, and evidence rules.
- Add unit tests that historical completed-task anchors can coexist with newer normative headings without breaking the active spec verifier.
- Integration tests are optional unless the verifier grows real collaborator dependencies.
- E2E tests are not required for this task because it hardens verification tooling rather than introducing a new runtime user flow.
- Run mutation testing because verification logic is easy to weaken accidentally.

## TDD Plan

- Start with a failing unit test for a dead spec anchor and a failing verifier expectation for active-scope reporting.

## Notes

- This task makes the planning/spec validation gate trustworthy before the final v0.1.0 closeout sweep.
- 2026-04-01T19:05:03Z: Hardened validate-state and verify --profile spec gates: replaced adapter-contract with agent-and-runtime-contract in required coverage, added host-prerequisites and compatibility-rules, implemented spec-ref alias resolution for historical completed-task references, added scope metadata to VerifyResult JSON output, added regression fixtures for dead-anchor, missing-file, and missing-coverage scenarios, strengthened hard-failure-gate documentation. Tests: go test -race (all pass), gremlins 85% efficacy, validate-state valid, verify --profile spec zero findings.
