---
id: TASK-053-bypass-user-config-when-cli-egress-is-explicit
title: Skip user-config loading when CLI egress fully determines resolution
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#generated-runtime-state
dependencies:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-041-opencode-proxy-user-config-allowlist-without-auth
    - TASK-042-opencode-auth-missing-actionable-error
updated_at: "2026-04-02T13:56:35Z"
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
- 2026-04-02T13:56:35Z: Guarded user-config loading in resolveAllowlistCore so open/none/proxy+CLI modes skip config entirely. Unit tests (4 cases), e2e tests (2 container tests), manual tests (5 steps) all pass. Mutation efficacy 85.63%.
