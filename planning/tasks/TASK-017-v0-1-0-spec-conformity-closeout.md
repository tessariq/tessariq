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
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-014-run-index-and-run-ref-resolution
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-016-v0-1-0-spec-conformity-verification
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
    - TASK-045-validate-index-entry-shape-before-resolution
    - TASK-046-unique-run-aware-last-run-resolution
    - TASK-047-promote-repo-local-evidence-path-validation
    - TASK-048-promote-manifest-run-identity-consistency
    - TASK-049-promote-require-diffstat-for-changed-runs
    - TASK-050-attach-preflight-git-prerequisite
    - TASK-051-attach-repo-local-evidence-path-validation
    - TASK-052-attach-run-id-evidence-path-consistency
    - TASK-053-bypass-user-config-when-cli-egress-is-explicit
    - TASK-054-reject-symlinked-external-task-paths
    - TASK-055-version-command-and-root-version-flag
    - TASK-056-enforce-network-none-for-egress-none
    - TASK-057-surface-diff-artifact-write-errors
    - TASK-058-reject-control-characters-in-allowlist-hosts
    - TASK-059-reject-leading-dot-allowlist-hosts
    - TASK-060-respect-grace-duration-during-container-shutdown
    - TASK-061-cleanup-worktrees-even-when-ownership-repair-fails
    - TASK-062-harden-squid-proxy-container-security
    - TASK-063-pin-squid-proxy-image-by-digest
    - TASK-064-enforce-host-port-pair-acls-in-squid-conf
    - TASK-065-support-ipv6-safe-egress-destination-parsing
    - TASK-066-preserve-binary-file-changes-in-diff-and-promote
    - TASK-067-cleanup-squid-resources-on-startup-failure
    - TASK-068-make-manifest-writes-atomic
    - TASK-069-reject-egress-allow-with-egress-open
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#release-intent
    - specs/tessariq-v0.1.0.md#product-intent
    - specs/tessariq-v0.1.0.md#host-prerequisites
    - specs/tessariq-v0.1.0.md#tessariq-version
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#tessariq-attach-run-ref
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
    - specs/tessariq-v0.1.0.md#success-metrics
updated_at: "2026-04-02T15:20:00Z"
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
- Every normative contract, acceptance scenario, failure-UX row, host-prerequisite contract, and evidence-compatibility rule in the active v0.1.0 spec is covered by tasks and implemented behavior.
- The closeout explicitly records each v0.1.0 success metric as met, not yet measurable, or follow-up required; it must not silently ignore the section.
- Regenerated verification artifacts and `planning/STATE.md` validation metadata point at the final passing sweep.
- The closeout sweep explicitly covers the v0.1.0 runtime-image contract, read-only supported-agent auth reuse, `--mount-agent-config`, agent-aware `auto` egress, and the `agent.json` plus `runtime.json` evidence split.
- The closeout sweep explicitly covers `tessariq version` and root `--version` as repository-independent CLI behavior.

## Test Expectations

- Add unit tests only if closeout uncovers a missing verifier assertion.
- Integration tests are optional unless the closeout workflow grows real collaborator dependencies.
- Add a full-pipeline e2e test covering the primary user journey end-to-end: `init -> create task -> run (detached) -> wait for completion -> promote -> verify branch, commit, trailers, and evidence artifacts`.
- Add error-path e2e tests for the two most common user-facing failures not covered by earlier tasks: dirty-repo rejection and invalid-task-path rejection.
- Add error-path e2e tests for missing host prerequisites: missing `git` for `init`/`run` preflight, missing `tmux` for attach/run session paths, and missing or unavailable `docker` for run/proxy paths.
- Add thin end-to-end coverage for the v0.1.0 auth/runtime failure paths: missing selected-agent binary, missing required supported-agent auth state, and unsupported writable auth-refresh expectation.
- Run mutation testing because the final gate should not rely on brittle or weakened verification logic.

## TDD Plan

- Start with the failing spec closeout sweep, then fix the highest-signal remaining gap before rerunning the gate.

## Notes

- This is the required final gate before considering `v0.1.0` complete.
