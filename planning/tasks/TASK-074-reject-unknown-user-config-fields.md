---
id: TASK-074-reject-unknown-user-config-fields
title: Reject unknown keys in user config instead of silently ignoring them
status: todo
priority: p3
depends_on:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-053-bypass-user-config-when-cli-egress-is-explicit
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-03T12:31:03Z"
areas:
    - config
    - networking
    - ux
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Unknown-field handling belongs in config parser unit coverage first.
    integration:
        required: false
        commands: []
        rationale: The fix is parser behavior rather than a new collaborator boundary.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Misconfigured user config is a real CLI failure mode and should be validated end to end.
    mutation:
        required: false
        commands: []
        rationale: Decoder strictness is a narrow parser change rather than branch-heavy logic.
    manual_test:
        required: true
        commands: []
        rationale: "Real config-typo behavior should be checked from the CLI to confirm the resulting guidance is actionable."
---

## Summary

`LoadUserConfig` currently uses permissive YAML unmarshalling, so misspelled keys are silently ignored and runs fall back to built-in allowlists. Switch to strict decoding so configuration typos fail loudly instead of widening egress unexpectedly.

## Supersedes

- BUG-040 from `planning/BUGS.md`.

## Acceptance Criteria

- Unknown top-level keys in `config.yaml` fail with an actionable error that identifies the config path.
- Valid `egress_allow` config continues to load unchanged.
- Missing config files and YAML syntax errors retain their existing behavior.
- Explicit CLI egress modes that bypass user config still do not read or fail on irrelevant config files.

## Test Expectations

- Add unit tests for strict rejection of unknown keys such as `egressAllow` and `egress_alow`.
- Add regression unit tests for valid config, missing config, and malformed-YAML behavior.
- Add an e2e check that a typoed user config now fails loudly when proxy defaults are consulted.

## TDD Plan

1. RED: add a failing parser test for an unknown YAML field.
2. GREEN: switch to strict decoding with clear error text.
3. GREEN: keep explicit CLI egress bypass behavior intact.

## Notes

- Likely files: `internal/run/userconfig.go` and `internal/run/userconfig_test.go`.
- Prefer the built-in strict decoder path from `gopkg.in/yaml.v3` over custom key validation.
