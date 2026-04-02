---
id: TASK-056-enforce-network-none-for-egress-none
title: Enforce Docker --net none for --egress none container runs
status: todo
priority: p0
depends_on:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-032-container-security-hardening
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-02T10:00:00Z"
areas:
    - networking
    - container
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Network-mode selection is branch logic that should be covered by focused unit tests.
    integration:
        required: false
        commands: []
        rationale: The core fix is container config wiring; Docker behavior is tested at the e2e level.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Real Docker networking posture must be validated with a running container.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Network isolation is security-critical and branch-heavy.
    manual_test:
        required: true
        commands: []
        rationale: Confirms the container has no internet access when --egress none is specified.
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
