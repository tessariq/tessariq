---
id: TASK-034-opencode-egress-allow-provider-bypass
title: Honor --egress-allow precedence when OpenCode provider host auto-resolution fails
status: todo
priority: p1
depends_on:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-025-opencode-agent-runtime-integration
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-31T20:30:00Z"
areas:
    - egress
    - opencode
    - cli
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Allowlist precedence and provider-resolution fallback are deterministic and unit-testable.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Must validate behavior with realistic auth/config file states.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: User-facing failure/success paths under `run --agent opencode --egress auto` are critical UX.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Conditional precedence logic is easy to break silently.
    manual_test:
        required: true
        commands: []
        rationale: Confirm guidance text and command-line workaround work in a realistic setup.
---

## Summary

OpenCode provider auto-resolution currently runs before allowlist precedence can apply explicit CLI `--egress-allow` destinations. This task makes explicit CLI allowlist input the authoritative override, allowing runs to proceed without provider auto-detection when users provide explicit destinations.

## Supersedes

- BUG-002 from `planning/BUGS.md`.

## Acceptance Criteria

- When one or more `--egress-allow` values are provided, resolved allowlist contains exactly those CLI destinations.
- For OpenCode in proxy/auto-resolved proxy mode, provider auto-resolution is skipped when explicit CLI allowlist is present.
- Failure guidance still triggers when no explicit CLI allowlist exists and provider host cannot be derived.
- `allowlist_source` and compiled allowlist evidence remain correct.
- Behavior for Claude Code and non-proxy modes is unchanged.

## Test Expectations

- Add unit tests covering OpenCode precedence matrix: CLI allowlist present vs absent, provider resolvable vs unresolvable.
- Add unit tests ensuring unresolved-provider error still appears when CLI allowlist is absent.
- Add integration/e2e tests proving explicit `--egress-allow` allows run setup without provider inference.
- Add regression tests for `--egress-no-defaults` interaction with explicit CLI allowlists.

## TDD Plan

1. RED: add failing test where OpenCode provider is unresolvable but CLI allowlist is explicitly provided.
2. RED: add failing test where provider is unresolvable and no CLI allowlist exists, expecting existing actionable error.
3. GREEN: modify allowlist resolution flow to short-circuit provider resolution when CLI allowlist is present.
4. GREEN: preserve existing provider-aware endpoint path when defaults are needed.
5. REFACTOR: keep resolution logic readable and separated from CLI command wiring.
6. GREEN: verify integration and e2e coverage.

## Notes

- Likely files: `cmd/tessariq/run.go`, `cmd/tessariq/run_test.go`, `internal/adapter/opencode/provider_test.go`, and e2e coverage in `cmd/tessariq/run_e2e_test.go`.
- Keep error wording aligned with current Failure UX language.
