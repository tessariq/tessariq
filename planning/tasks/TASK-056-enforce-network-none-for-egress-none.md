---
id: TASK-056-enforce-network-none-for-egress-none
title: Enforce Docker --net none for --egress none container runs
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#networking-and-egress
dependencies:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-032-container-security-hardening
updated_at: "2026-04-02T08:44:18Z"
---

## Summary

`--egress none` must pass `--net none` to `docker create` so the container gets loopback-only networking. Currently `NetworkName` stays empty for non-proxy modes, causing Docker to default to the bridge network with full internet access.

## Supersedes

- BUG-023 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq run --egress none <task-path>` produces a container on Docker's `none` network (loopback only, no external connectivity).
- `--egress open` remains on the default Docker bridge network (unchanged behavior).
- `--egress proxy` remains on the per-run proxy network (unchanged behavior).
- `--egress auto` resolving to `none` also gets `--net none`.
- The resolved network mode is auditable in evidence artifacts.

## Test Expectations

- Add unit tests asserting that the container config sets `NetworkName` to `"none"` when `resolvedEgress == "none"`.
- Add unit tests confirming `open` and `proxy` modes are unchanged.
- Add e2e coverage verifying the container cannot reach external hosts under `--egress none`.
- Add regression coverage for proxy mode network isolation.

## TDD Plan

1. RED: add a failing test that asserts `NetworkName == "none"` when resolved egress is `none`.
2. GREEN: set `networkName` to `"none"` in the egress-none code path.
3. REFACTOR: keep the egress-to-network mapping explicit and centralized.
4. GREEN: verify proxy and open modes are unaffected.

## Notes

- Likely files: `cmd/tessariq/run.go`, `internal/adapter/factory.go`, `internal/container/process.go`.
- Docker's built-in `none` network provides loopback only — no additional network creation or cleanup needed.
- The fix is a one-line network-name assignment but the security impact is critical.
- 2026-04-02T08:44:18Z: Enforced --net none for --egress none. Unit tests cover all egress-to-network mappings. E2E test confirms container has no external connectivity. Mutation testing at 85.52% efficacy. Manual tests: 5/5 pass.
