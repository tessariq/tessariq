---
id: TASK-084-agent-auto-update
title: Agent auto-update via cache-aware init container
status: todo
priority: p1
depends_on:
    - TASK-008-adapter-contract-and-adapter-json
    - TASK-009-claude-code-adapter
    - TASK-010-opencode-adapter
    - TASK-021-reference-runtime-image-and-docs
    - TASK-024-claude-code-agent-runtime-integration
    - TASK-025-opencode-agent-runtime-integration
    - TASK-027-container-lifecycle-and-mount-isolation
    - TASK-037-prestart-agent-binary-validation
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-04-07T20:30:00Z"
areas:
    - adapters
    - container
    - evidence
    - cli
    - spec
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Adapter contract methods, runtime evidence serialization, and init container orchestration logic all need unit-level coverage.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Init container lifecycle, cache volume mounting, and PATH layering involve real Docker operations.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: End-to-end tests must verify that runtime.json contains agent_update evidence and that the updated agent binary is used.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Init container skip/fallback logic is branchy and must survive mutation testing.
    manual_test:
        required: true
        commands: []
        rationale: Real run should demonstrate agent update output, runtime.json evidence, and fallback behavior.
---

## Summary

Add a cache-aware init container mechanism that automatically updates agents to the latest version at container start. The init container runs before the agent container, installs the latest agent binary into a persistent global cache at `~/.tessariq/agent-cache/<agent>/`, and the agent container mounts the cache read-only with PATH layering so updated binaries shadow the baked version. The baked version in the runtime image is the version floor — no downgrades. Update failures fall back to the baked version with a warning.

## Acceptance Criteria

- The `Agent` interface gains `UpdateCommand(prefix string) []string` and `VersionCommand() []string` methods.
- Claude Code adapter returns `["npm", "install", "--global", "--prefix", prefix, "@anthropic-ai/claude-code@latest"]` for `UpdateCommand` and `["claude", "--version"]` for `VersionCommand`.
- OpenCode adapter returns `["npm", "install", "--global", "--prefix", prefix, "opencode-ai@latest"]` for `UpdateCommand` and `["opencode", "--version"]` for `VersionCommand`.
- By default, `tessariq run` runs an init container (same image, `--rm`, root user, 120s timeout) that executes the adapter's `UpdateCommand("/cache")` with the cache dir bind-mounted at `/cache`.
- The init container receives no auth, config, or workdir mounts.
- On init success, the agent container mounts `~/.tessariq/agent-cache/<agent>/` read-only at `/cache` and gets `PATH=/cache/bin:$PATH`.
- On init failure, Tessariq logs a warning, records the failure in `runtime.json`, and proceeds with the baked agent version.
- `--no-update-agent` skips the init phase entirely and uses the baked version.
- `runtime.json` includes an `agent_update` field recording: `attempted`, `success`, `cached_version`, `baked_version`, `elapsed_ms`, and `error`.
- User-facing output shows update progress: `Updating claude-code agent... done (2.1.92 -> 2.3.0, 4.2s)` or failure message.
- The agent cache directory `~/.tessariq/agent-cache/<agent>/` is created on first use and persists across runs.

## Test Expectations

- Unit tests for `UpdateCommand` and `VersionCommand` return values in both adapter packages.
- Unit tests for `RuntimeInfo` JSON serialization including `AgentUpdate` struct with all states (attempted/skipped, success/failure).
- Unit tests for init container skip logic (`--no-update-agent`, nil `UpdateCommand`).
- Integration tests using Testcontainers: init container installs agent into cache prefix; PATH layering makes updated binary available; failure scenario falls back to baked version.
- E2e test: full `tessariq run` with update enabled produces `runtime.json` with `agent_update` evidence.
- E2e test: `tessariq run --no-update-agent` produces `runtime.json` with `agent_update.attempted: false`.
- Manual test: run with and without `--no-update-agent`; verify user-facing output; inspect `runtime.json`; clear cache and verify re-download; disconnect network and verify fallback.

## TDD Plan

1. RED: add `UpdateCommand` and `VersionCommand` to `Agent` interface; existing adapter tests fail to compile.
2. GREEN: implement methods in Claude Code and OpenCode adapters.
3. RED: add `AgentUpdate` struct and field to `RuntimeInfo`; add serialization tests expecting `agent_update` in JSON output.
4. GREEN: implement `AgentUpdate` in `runtime.go` and wire into `NewRuntimeInfo`.
5. RED: add init container orchestration tests (skip logic, timeout, success/failure paths).
6. GREEN: implement `internal/container/init.go` with `RunInitContainer` function.
7. RED: add integration tests for cache volume mounting and PATH layering.
8. GREEN: wire init phase into `NewProcess` / `run.go`, add `--no-update-agent` flag.
9. REFACTOR: clean up, verify all paths produce correct evidence.
10. VERIFY: run full test suites, mutation testing, and manual testing.

## Notes

- The init container runs on the default Docker network, not the proxy network — egress restrictions apply to the agent runtime, not to package installation.
- The init container runs as root for npm global install access; the agent container still runs as the `tessariq` user.
- This feature is compatible with the v0.3.0 `tessariq runtime bake` direction — baked images provide a floor, auto-update provides freshness.
- Future work may add user-configurable version pinning or update channels; the current design uses `@latest`.
- Design doc: `/home/felix/.claude/plans/robust-stargazing-cloud.md`
