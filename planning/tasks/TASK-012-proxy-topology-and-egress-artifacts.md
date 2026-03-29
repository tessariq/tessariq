---
id: TASK-012-proxy-topology-and-egress-artifacts
title: Implement proxy topology and egress evidence artifacts
status: todo
priority: p1
depends_on:
  - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
  - TASK-011-egress-mode-resolution-and-manifest-recording
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
  - specs/tessariq-v0.1.0.md#networking-and-egress
  - specs/tessariq-v0.1.0.md#evidence-contract
  - specs/tessariq-v0.1.0.md#acceptance-scenarios
  - specs/tessariq-v0.1.0.md#failure-ux
updated_at: 2026-03-29T12:06:20Z
areas:
  - networking
  - proxy
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
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Rule compilation and allowlist enforcement are high-value mutation targets.
---

## Summary

Implement proxy mode runtime topology, compiled allowlists, and egress event evidence.

## Acceptance Criteria

- Proxy mode integrates with the runner/container lifecycle and enforces host:port allowlists with default port `443`.
- `egress.compiled.yaml` is emitted in proxy mode with `schema_version`, `allowlist_source`, and fully resolved destination `host` and `port` entries.
- `egress.events.jsonl` is emitted only in proxy mode and records blocked attempts alongside the resolved allowlist context.
- HTTPS and WSS CONNECT-style traffic is supported through the allowlisted proxy path.
- Proxy evidence records both allowlist provenance and the fully resolved destinations without re-derivation.
- Blocked-destination failures tell the user which `host:port` was blocked and how to allow it through user config or CLI flags, or rerun with explicit open egress.

## Test Expectations

- Add unit tests for allowlist compilation and manifest/proxy configuration rendering.
- Add integration tests for proxy-mode runtime behavior using Testcontainers-backed collaborators only.
- Add a thin e2e proxy-mode flow once run execution is end-to-end capable.
- Run mutation testing because policy logic is safety-critical.

## TDD Plan

- Start with a failing unit test for allowlist compilation and a failing integration test for proxy-mode egress enforcement.

## Notes

- Keep low-level proxy topology implementation details informative, not user-contract primary.
