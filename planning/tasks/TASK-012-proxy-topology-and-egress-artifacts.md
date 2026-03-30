---
id: TASK-012-proxy-topology-and-egress-artifacts
title: Implement proxy topology and provider-aware egress evidence artifacts
status: todo
priority: p1
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-011-egress-mode-resolution-and-manifest-recording
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#host-prerequisites
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-30T23:05:00Z"
areas:
    - networking
    - proxy
    - agents
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Allowlists and compiled rule rendering should begin with unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Proxy networking is a real container boundary and must use Testcontainers-backed integration coverage only.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: The user-visible proxy mode should receive thin end-to-end verification once the run flow is complete.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Rule compilation and allowlist enforcement are high-value mutation targets.
    manual_test:
        required: true
        commands: []
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
---

## Summary

Implement proxy mode runtime topology, compiled allowlists, and egress event evidence for the selected supported agent, including provider-aware OpenCode behavior.

## Acceptance Criteria

- Proxy mode integrates with the runner/container lifecycle and enforces host:port allowlists with default port `443`.
- Proxy mode preflights Docker prerequisites (binary availability and daemon reachability) before attempting container/network setup.
- `egress.compiled.yaml` is emitted in proxy mode with `schema_version`, `allowlist_source`, and fully resolved destination `host` and `port` entries.
- `egress.events.jsonl` is emitted only in proxy mode and records blocked attempts alongside the resolved allowlist context.
- HTTPS and WSS CONNECT-style traffic is supported through the allowlisted proxy path.
- Proxy evidence records both allowlist provenance and the fully resolved destinations without re-derivation.
- The proxy topology works with Claude Code's fixed first-party endpoints under `--egress auto`.
- The proxy topology works with OpenCode's provider-aware endpoint profile under `--egress auto`, including `models.dev:443` and the resolved provider host on `443`.
- OpenCode `--egress auto` never silently falls back to broad network access when the provider host cannot be resolved.
- Blocked-destination failures tell the user which `host:port` was blocked and how to allow it through user config or CLI flags, or rerun with explicit open egress.
- Missing/unavailable Docker failures tell the user Docker is required for proxy mode and provide actionable retry guidance.

## Test Expectations

- Add unit tests for allowlist compilation and manifest/proxy configuration rendering.
- Add unit tests for Docker prerequisite checks and user-facing missing/unavailable Docker guidance.
- Add integration tests for proxy-mode runtime behavior using Testcontainers-backed collaborators only.
- Add integration tests for Docker-daemon-unavailable failure handling.
- Add integration tests that Claude Code can reach its maintained allowlisted endpoints and is blocked from non-allowlisted destinations.
- Add integration tests that OpenCode can reach `models.dev` and the resolved provider host while still being blocked from non-allowlisted destinations.
- Add integration coverage that unresolved OpenCode provider-host cases fail before container start rather than broadening egress.
- Add a thin e2e proxy-mode flow once run execution is end-to-end capable.
- Run mutation testing because policy logic is safety-critical.

## TDD Plan

- Start with a failing unit test for allowlist compilation and a failing integration test for provider-aware proxy-mode egress enforcement.

## Notes

- Keep low-level proxy topology implementation details informative, not user-contract primary.
