---
id: TASK-025-opencode-agent-runtime-integration
title: Integrate OpenCode with the v0.1.0 agent and runtime model
status: todo
priority: p1
depends_on:
    - TASK-021-reference-runtime-image-and-docs
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-023-supported-agent-auth-mounts
    - TASK-026-mount-agent-config-flag-and-config-dir-mounts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-30T23:05:00Z"
areas:
    - agents
    - opencode
    - runtime
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Command construction, option application, and runtime validation should start with unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Real agent invocation and container-visible runtime validation require Testcontainers-backed coverage.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Supported-agent behavior is part of the end-to-end user journey.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Option mapping and failure guidance are branch-heavy.
    manual_test:
        required: true
        commands: []
        rationale: Real local auth reuse for OpenCode must be validated manually.
---

## Summary

Integrate `opencode` with the v0.1.0 agent/runtime model, including runtime-binary validation, read-only auth reuse, and the new evidence contract.

## Acceptance Criteria

- `agent.json` records `agent=opencode` and the requested/applied option semantics required by the active spec.
- OpenCode integrates cleanly with the run lifecycle.
- Tessariq validates that the `opencode` binary is present in the resolved runtime image before agent start.
- Missing-OpenCode-binary failures identify the missing `opencode` binary and tell the user to use a compatible runtime image or `--image` override.
- OpenCode works with the supported read-only auth-mount contract using `~/.local/share/opencode/auth.json`.
- OpenCode uses the provider-aware `--egress auto` profile: `models.dev:443`, the resolved provider base-URL host on `443`, and `opencode.ai:443` only when the resolved provider or auth flow requires it.
- When the OpenCode provider host cannot be resolved from available config and auth state under `--egress auto`, Tessariq fails before container start with actionable guidance.

## Test Expectations

- Add unit tests for command/option translation, `opencode` runtime-binary validation, and provider-host resolution.
- Add integration tests for real agent invocation using Testcontainers-backed collaborators only.
- Add integration tests for missing-binary, missing-auth, and unresolved-provider-host error handling.
- Add integration tests for agent process crash mid-run.
- Add a thin e2e run path once the agent is wired into run execution.
- Run mutation testing because option translation is branchy.

## TDD Plan

- Start with a failing unit test for OpenCode option translation, missing-`opencode`-binary validation, and provider-host resolution.

## Notes

- This task supersedes the old adapter-specific implementation task without rewriting that completed task.
