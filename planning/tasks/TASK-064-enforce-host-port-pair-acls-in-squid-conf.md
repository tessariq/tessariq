---
id: TASK-064-enforce-host-port-pair-acls-in-squid-conf
title: Enforce exact host-port pairs in generated Squid ACLs
status: done
priority: p0
depends_on:
    - TASK-012-proxy-topology-and-egress-artifacts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-03T09:08:44Z"
areas:
    - proxy
    - networking
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Squid config generation is deterministic text output and should be locked down with focused unit coverage.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The fix must be validated against a real Squid container to prove unintended host-port pairs are blocked.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: This is a user-visible and security-critical proxy guarantee.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: ACL-composition logic is branch-heavy and safety-critical.
    manual_test:
        required: true
        commands: []
        rationale: Confirms proxy-mode runs can reach only the exact configured host-port pairs.
---

## Summary

`GenerateSquidConf` currently builds one shared host ACL and one shared port ACL, which lets any allowed host be reached on any allowed port. Rework the generated rules so every allowlist entry is enforced as an exact host-port pair.

## Supersedes

- BUG-031 from `planning/BUGS.md`.

## Acceptance Criteria

- A configured allowlist entry authorizes only its own host-port pair.
- Mixed-port allowlists no longer permit the unintended host-port cross-product.
- Generated Squid config remains deterministic and auditable.
- Existing single-destination and same-port multi-destination behavior remains unchanged apart from stricter enforcement.

## Test Expectations

- Add unit tests for mixed-port allowlists proving the generated config expresses per-destination matching.
- Add integration coverage that attempts an unintended host-port combination through Squid and verifies it is denied.
- Add e2e regression coverage for a valid proxy-mode run using multiple destinations.

## TDD Plan

1. RED: add config-generation and runtime tests that expose the current cross-product.
2. GREEN: emit per-destination ACL rules rather than shared host and port ACL buckets.
3. REFACTOR: keep generated names deterministic so evidence diffs stay readable.

## Notes

- Likely files: `internal/proxy/squidconf.go`, `internal/proxy/squidconf_test.go`, and proxy integration/e2e coverage.
- Favor the simplest native Squid rule structure that preserves exact pair semantics without introducing helper services.
- 2026-04-03T09:08:44Z: Per-port ACL grouping eliminates cross-product. Unit tests (8+1 new), integration test (cross-port denial), e2e test (multi-destination), mutation 85.58%, manual test 4/4 pass.
