---
id: TASK-011-egress-mode-resolution-and-manifest-recording
title: Resolve egress modes, provider-aware allowlists, and manifest recording
status: done
priority: p1
depends_on:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-024-claude-code-agent-runtime-integration
    - TASK-025-opencode-agent-runtime-integration
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#generated-runtime-state
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: "2026-03-31T09:33:02Z"
areas:
    - networking
    - evidence
    - agents
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Egress resolution, allowlist provenance, and manifest-field population should be unit-tested first.
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
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Mode resolution and built-in allowlist selection are mutation-prone.
    manual_test:
        required: true
        commands: []
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
---

## Summary

Implement requested-versus-resolved egress mode handling, user-level allowlist defaulting, and per-agent or provider-aware allowlist resolution for the selected supported agent.

## Acceptance Criteria

- `auto` resolves to `proxy` for the supported first-party agents.
- `open` requires explicit unsafe opt-in.
- Requested and resolved egress modes are preserved in `manifest.json`.
- `manifest.json` uses `agent`, not `adapter`.
- `allowlist_source` is recorded as exactly one of `cli`, `user_config`, or `built_in`.
- User-level config is read from the documented XDG/default path locations for proxy/auto allowlist defaults only, and CLI flags remain the per-run source of truth.
- `--egress-no-defaults` discards built-in and user-configured defaults before later CLI allowlist entries are applied.
- Allowlist precedence follows CLI entries first, then user-level config, then the built-in profile.
- The built-in Tessariq allowlist profile includes the baseline package-manager destinations defined by the active v0.1.0 spec.
- The Claude Code built-in profile includes exactly `api.anthropic.com:443`, `claude.ai:443`, and `platform.claude.com:443` in addition to the baseline package-manager profile.
- The OpenCode built-in profile is provider-aware: it includes `models.dev:443`, the resolved provider base-URL host on `443`, and `opencode.ai:443` only when the resolved configuration requires an OpenCode-hosted provider or auth flow.
- When OpenCode is selected and the provider host cannot be resolved from available config and auth state under `--egress auto`, Tessariq fails before container start with actionable guidance.
- `manifest.json` records `requested_egress_mode`, `resolved_egress_mode`, and `allowlist_source`.

## Test Expectations

- Add unit tests for mode resolution, aliases, XDG/default config discovery, allowlist precedence, `--egress-no-defaults`, and manifest recording.
- Add unit tests for config file error handling: malformed YAML (graceful failure with user guidance), unreadable file permissions, `$XDG_CONFIG_HOME` pointing to non-existent directory, and config files with unknown keys (forward compatibility).
- Add unit tests for allowlist entry validation: invalid hostnames, non-numeric ports, and empty allowlist in proxy mode.
- Add unit tests that Claude Code endpoint profiles are included only for Claude Code.
- Add unit tests that OpenCode allowlists are derived from the resolved provider host and that unresolved-provider cases fail cleanly under `--egress auto`.
- Integration tests are deferred until proxy topology exists.
- E2E tests are deferred until runtime networking is active.
- Run mutation testing because the resolution logic is branch-heavy.

## TDD Plan

- Start with a failing unit test for `auto` resolution, provider-aware OpenCode allowlist derivation, and unsafe-open validation.

## Notes

- Keep allowlist normalization, provider-host derivation, and provenance explicit; proxy transport details stay in `TASK-012`.
- 2026-03-31T09:33:02Z: All acceptance criteria verified. Unit tests: go test ./... green. Mutation testing: 90.91% efficacy (>70% threshold). Manual testing: 12/12 pass. Evidence: planning/artifacts/manual-test/TASK-011-egress-mode-resolution-and-manifest-recording/20260331T093024Z/, planning/artifacts/verify/task/TASK-011-egress-mode-resolution-and-manifest-recording/20260331T093007Z/
