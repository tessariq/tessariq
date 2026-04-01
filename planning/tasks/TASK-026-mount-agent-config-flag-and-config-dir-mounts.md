---
id: TASK-026-mount-agent-config-flag-and-config-dir-mounts
title: Add --mount-agent-config and read-only default config-dir mounts
status: done
priority: p1
depends_on:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-023-supported-agent-auth-mounts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-31T08:15:44Z"
areas:
    - cli
    - agents
    - runtime
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Flag wiring and mount-policy recording should start with unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Container-visible config-dir mounts need Testcontainers-backed verification.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: Full end-to-end behavior is already exercised by the supported-agent tasks.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Config-mount guardrails and failure behavior are branch-prone.
    manual_test:
        required: true
        commands: []
        rationale: The opt-in config-dir mount UX should be validated directly.
---

## Summary

Add `--mount-agent-config` so users can opt in to read-only mounting of supported agents' default config directories without exposing host `HOME`.

## Acceptance Criteria

- A new `--mount-agent-config` boolean flag exists on `tessariq run` with default `false`.
- When the flag is not set, Tessariq mounts only the required supported-agent auth files or directories.
- When the flag is set, Tessariq additionally mounts the selected supported agent's default config directories read-only:
  - Claude Code: `~/.claude/` to `$HOME/.claude/`
  - OpenCode: `~/.config/opencode/` to `$HOME/.config/opencode/`
- Tessariq does not mount arbitrary host-home paths as a side effect of the flag.
- `runtime.json` records `agent_config_mount` as `disabled` or `enabled` and `agent_config_mount_status` as exactly one of `disabled`, `mounted`, `missing_optional`, or `unreadable_optional`.
- Missing or unreadable optional config dirs do not leak secrets, warn on stderr, are recorded in `runner.log`, and do not fail the run when required auth mounts are valid.

## Test Expectations

- Add unit tests for flag parsing, defaulting, and `runtime.json` recording.
- Add integration tests that the expected read-only config-dir mounts are present only when the flag is enabled.
- Add integration tests that host `HOME` is still not exposed when the flag is enabled.
- Add integration tests that missing or unreadable optional config dirs produce warnings and `runtime.json` status without failing the run when required auth mounts are valid.
- Run mutation testing because the mount-policy branching is easy to weaken accidentally.

## TDD Plan

- Start with a failing unit test for `--mount-agent-config` defaulting and `runtime.json` emission.

## Notes

- This flag is intentionally separate from the always-on required auth reuse flow.
- 2026-03-31T08:15:44Z: Added --mount-agent-config flag, DiscoverConfigDirs in authmount, updated adapter.NewProcess signature, runtime.json status values, integration tests with Testcontainers. Mutation efficacy 90.80%. Manual test 8/8 pass. Local-only manual-test and verification artifacts generated.
