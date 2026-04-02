---
id: TASK-062-harden-squid-proxy-container-security
title: Apply capability dropping and no-new-privileges to the Squid proxy container
status: todo
priority: p1
depends_on:
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-032-container-security-hardening
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-02T14:59:17Z"
areas:
    - proxy
    - container
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Docker create arg construction should be covered at unit level first.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The proxy container's effective security posture must be verified against real Docker state.
    e2e:
        required: false
        commands: []
        rationale: Focused integration inspection is sufficient for the proxy hardening path.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Security-flag injection should not silently weaken.
    manual_test:
        required: true
        commands: []
        rationale: Confirms the proxy container matches the intended Docker security posture under `docker inspect`.
---

## Summary

The agent container already drops all capabilities and disables privilege escalation, but the Squid proxy container does not. Harden the proxy container with the same baseline restrictions so the egress boundary is not the weakest container in the topology.

## Supersedes

- BUG-029 from `planning/BUGS.md`.

## Acceptance Criteria

- `StartSquid` includes `--cap-drop=ALL` and `--security-opt=no-new-privileges` in its `docker create` call.
- Proxy-mode runs still start and pass readiness checks with the hardened container.
- Hardening changes are limited to the proxy container and do not affect repair-container behavior.
- Integration coverage verifies the effective HostConfig values, not only string assembly.

## Test Expectations

- Add unit tests for Squid `docker create` arg construction.
- Add integration coverage that inspects the created Squid container and verifies `CapDrop` and `NoNewPrivileges`.
- Add regression coverage that proxy startup still succeeds after hardening.

## TDD Plan

1. RED: add a failing test for missing Squid security flags.
2. GREEN: add the hardening flags to `StartSquid`.
3. GREEN: verify readiness and teardown still succeed with the hardened container.

## Notes

- Likely files: `internal/proxy/squid.go` and proxy integration tests.
- Keep the proxy container otherwise behavior-preserving; this task is about baseline containment, not broader resource limits.
