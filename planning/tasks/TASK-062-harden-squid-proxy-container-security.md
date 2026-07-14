---
id: TASK-062-harden-squid-proxy-container-security
title: Apply capability dropping and no-new-privileges to the Squid proxy container
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#networking-and-egress
dependencies:
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-032-container-security-hardening
updated_at: "2026-04-03T10:38:21Z"
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
- 2026-04-03T10:38:21Z: Squid proxy container hardened with --cap-drop=ALL --cap-add=SETGID --cap-add=SETUID --security-opt=no-new-privileges. Fixed squid.conf temp file permissions (0644) for compatibility. Fixed CopyAccessLog to exec as proxy user. Tests: 4 unit, 1 integration (HostConfig inspection), 3 existing integration regression. Mutation: 85.64% efficacy.
