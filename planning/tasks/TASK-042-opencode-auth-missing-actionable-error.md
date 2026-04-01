---
id: TASK-042-opencode-auth-missing-actionable-error
title: Surface actionable auth-missing guidance for OpenCode provider resolution
status: todo
priority: p1
depends_on:
    - TASK-023-supported-agent-auth-mounts
    - TASK-041-opencode-proxy-user-config-allowlist-without-auth
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#failure-ux
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
updated_at: "2026-04-01T13:05:00Z"
areas:
    - egress
    - opencode
    - error-handling
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Error typing and message mapping should be asserted with deterministic unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Auth discovery and provider resolution sequencing needs realistic coverage.
    e2e:
        required: false
        commands: []
        rationale: Integration coverage is sufficient for this focused pre-start failure UX.
    mutation:
        required: false
        commands: []
        rationale: Existing mutation scope can stay unchanged for this narrow UX mapping fix.
    manual_test:
        required: true
        commands: []
        rationale: Confirm final CLI error wording is actionable and references local authentication.
---

## Summary

When OpenCode provider resolution cannot find auth state, Tessariq currently surfaces raw filesystem errors instead of the actionable auth guidance required by the failure UX contract.

## Supersedes

- BUG-010 from `planning/BUGS.md`.

## Acceptance Criteria

- Missing OpenCode auth state during provider-resolution paths returns an actionable error that identifies missing local auth and tells the user to authenticate OpenCode first.
- Raw `open ... no such file or directory` messages are not surfaced directly to end users for this case.
- Failure still occurs before agent start when provider information is required and unavailable.
- Existing valid auth flows remain unchanged.

## Test Expectations

- Add unit tests for auth-missing error mapping in allowlist/provider resolution.
- Add integration regression confirming user-facing error message when auth is absent and built-in endpoint derivation is required.
- Verify no regression in paths where auth exists or where provider resolution is intentionally skipped.

## TDD Plan

1. RED: add failing test expecting actionable auth-missing message.
2. GREEN: convert missing-auth provider read failures to the existing auth-missing error type/message path.
3. REFACTOR: keep error wrapping contextual and `%w`-compatible.
4. GREEN: re-run targeted suites.

## Notes

- Consider ordering/flow changes between allowlist resolution and `authmount.Discover` if that yields cleaner UX consistency.
## Non-goals

- Do not make missing auth non-fatal when OpenCode built-in provider endpoint derivation is required.
- Do not duplicate or re-implement user-config allowlist bypass logic from TASK-041.

