---
id: TASK-011-egress-mode-resolution-and-manifest-recording
title: Resolve egress modes and record them in manifest output
status: todo
priority: p1
depends_on:
  - TASK-002-run-cli-flags-and-manifest-bootstrap
  - TASK-008-adapter-contract-and-adapter-json
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
  - specs/tessariq-v0.1.0.md#generated-runtime-state
  - specs/tessariq-v0.1.0.md#tessariq-run-task-path
  - specs/tessariq-v0.1.0.md#networking-and-egress
  - specs/tessariq-v0.1.0.md#evidence-contract
  - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: 2026-03-29T00:00:00Z
areas:
  - networking
  - evidence
verification:
  unit:
    required: true
    commands:
      - go test ./...
    rationale: Egress resolution and manifest-field population should be unit-tested first.
  integration:
    required: false
    commands:
      - go test -tags=integration ./...
    rationale: Integration coverage can wait until proxy topology and runtime networking are implemented.
  e2e:
    required: false
    commands:
      - go test -tags=e2e ./...
    rationale: Full user-flow verification belongs with the proxy runtime task.
  mutation:
    required: true
    commands:
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Mode resolution and unsafe-open gating are mutation-prone.
---

## Summary

Implement requested-versus-resolved egress mode handling, user-level allowlist defaulting, and manifest recording.

## Acceptance Criteria

- `auto` resolves to `proxy` for the supported first-party adapters.
- `open` requires explicit unsafe opt-in.
- Requested and resolved egress modes are preserved in `manifest.json`.
- User-level config is read from the documented XDG/default path locations for proxy/auto allowlist defaults only.
- `--egress-allow-reset` discards built-in and user-configured defaults before later CLI allowlist entries are applied.
- Allowlist precedence follows CLI entries first, then user-level config, then the built-in profile.
- `manifest.json` records `requested_egress_mode`, `resolved_egress_mode`, and `allowlist_source`.

## Test Expectations

- Add unit tests for mode resolution, aliases, XDG/default config discovery, allowlist precedence, `--egress-allow-reset`, and manifest recording.
- Integration tests are deferred until proxy topology exists.
- E2E tests are deferred until runtime networking is active.
- Run mutation testing because the resolution logic is branch-heavy.

## TDD Plan

- Start with a failing unit test for `auto` resolution and unsafe-open validation.

## Notes

- Keep allowlist normalization and provenance explicit; proxy transport details stay in `TASK-012`.
