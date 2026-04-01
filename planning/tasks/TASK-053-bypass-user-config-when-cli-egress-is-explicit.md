---
id: TASK-053-bypass-user-config-when-cli-egress-is-explicit
title: Skip user-config loading when CLI egress fully determines resolution
status: todo
priority: p1
depends_on:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-041-opencode-proxy-user-config-allowlist-without-auth
    - TASK-042-opencode-auth-missing-actionable-error
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#generated-runtime-state
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-01T20:03:47Z"
areas:
    - networking
    - config
    - cli
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Precedence and short-circuit behavior belong in unit coverage first.
    integration:
        required: false
        commands: []
        rationale: The core fix is config-loading control flow rather than a new collaborator boundary.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: This is a real user-facing run failure mode caused by malformed user config.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Config precedence and bypass logic are branch-heavy.
    manual_test:
        required: true
        commands: []
        rationale: Confirms explicit CLI egress choices no longer fail on irrelevant malformed user config.
---

## Summary

`resolveAllowlistCore()` currently loads user config before checking whether CLI inputs already fully determine the run's egress behavior. Skip config loading when explicit CLI egress settings make user defaults irrelevant.

## Supersedes

- BUG-019 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq run --egress open ...` does not read or parse user config.
- `tessariq run --egress none ...` does not read or parse user config.
- `tessariq run --egress proxy --egress-allow ...` does not read or parse user config because the CLI allowlist fully determines the resolved allowlist.
- Malformed or unreadable user config only fails runs when that config is actually needed to determine proxy/auto defaults.
- Existing precedence remains unchanged when user config is relevant.

## Test Expectations

- Add unit tests proving malformed config is ignored for explicit `open`, `none`, and CLI-allowlist-driven proxy runs.
- Add regression unit tests proving malformed config still fails when user-config defaults are actually consulted.
- Add e2e coverage for malformed config plus explicit CLI egress settings.

## TDD Plan

1. RED: add failing tests for malformed config under explicit `open` and explicit CLI allowlist cases.
2. GREEN: short-circuit user-config discovery and parsing when CLI inputs already determine the outcome.
3. REFACTOR: keep allowlist resolution readable and explicit about when config is consulted.
4. GREEN: verify auth-missing and user-config precedence behavior still works in relevant paths.

## Notes

- Likely files: `cmd/tessariq/run.go` and allowlist resolution tests.
- Preserve the existing behavior for proxy/auto runs that genuinely depend on user-config defaults.
