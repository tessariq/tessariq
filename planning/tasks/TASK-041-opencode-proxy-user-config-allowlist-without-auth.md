---
id: TASK-041-opencode-proxy-user-config-allowlist-without-auth
title: Skip OpenCode provider resolution when user-config allowlist already resolves proxy egress
status: todo
priority: p0
depends_on:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-023-supported-agent-auth-mounts
    - TASK-034-opencode-egress-allow-provider-bypass
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-01T13:05:00Z"
areas:
    - egress
    - opencode
    - config
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Allowlist precedence and provider-resolution guards should be validated via deterministic unit cases.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: OpenCode auth/config path interactions must be validated with realistic filesystem/layout collaborators.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Confirm end-user run behavior with user config defaults and missing auth.json.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Precedence branches are easy to regress and should retain mutation resistance.
    manual_test:
        required: true
        commands: []
        rationale: Validate practical CLI flow using only `~/.config/tessariq/config.yaml` allowlist defaults.
---

## Summary

OpenCode proxy runs currently resolve provider host from auth/config even when a user-config default allowlist already fully determines egress destinations. This creates an unnecessary hard dependency on `auth.json` and fails otherwise-valid runs.

## Supersedes

- BUG-009 from `planning/BUGS.md`.

## Acceptance Criteria

- For `--agent opencode` with proxy egress and no CLI `--egress-allow`, if user config provides `egress_allow`, run allowlist resolution succeeds without reading OpenCode auth/provider state.
- Missing `~/.local/share/opencode/auth.json` does not fail the run when user-config allowlist is present.
- Precedence remains unchanged: CLI allowlist > user config allowlist > built-in allowlist.
- When neither CLI nor user-config allowlist exists, provider resolution still runs for built-in OpenCode endpoints.

## Test Expectations

- Add unit tests for `resolveAllowlistCore` covering:
  - user-config allowlist present + missing auth => success with `allowlist_source=user_config`.
  - no user-config allowlist + missing auth => existing failure path.
  - CLI allowlist present => provider resolution skipped.
- Add integration/e2e regression for OpenCode run with user-config allowlist and absent auth file.

## TDD Plan

1. RED: add failing test proving user-config allowlist path still tries provider resolution.
2. GREEN: update provider-resolution guard to respect user-config precedence.
3. REFACTOR: keep resolution logic readable and centralized.
4. GREEN: re-run full allowlist-related tests.

## Notes

- Likely files: `cmd/tessariq/run.go` and allowlist resolution tests.
## Non-goals

- Do not change auth-missing error wording for cases where provider resolution is genuinely required (tracked by TASK-042).
- Do not alter allowlist precedence semantics beyond skipping unnecessary provider resolution.

