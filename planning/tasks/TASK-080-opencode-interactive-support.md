---
id: TASK-080-opencode-interactive-support
title: Support --interactive flag for OpenCode adapter
status: todo
priority: p1
depends_on:
    - TASK-033-opencode-interactive-request-recording
    - TASK-078-fix-interactive-attach-double-pty-and-task-passthrough
    - TASK-079-forward-model-flag-and-adapter-interface
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-04-06T00:00:00Z"
areas:
    - adapter
    - opencode
    - cli
    - interactive
verification:
    unit:
        required: true
        commands:
            - go test ./internal/adapter/...
            - go test ./...
        rationale: buildApplied change and warning removal are small but branch-sensitive; unit tests pin the new semantics.
    integration:
        required: false
        commands: []
        rationale: No new subsystem boundaries introduced; existing integration tests cover the agent-agnostic interactive runtime.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Existing e2e tests assert opencode interactive is not applied; they must be updated and re-verified.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: The applied-flag branch is trivially mutatable and guards user-visible evidence output.
    manual_test:
        required: true
        commands: []
        rationale: The opencode TUI in a Docker container with TTY must be validated manually — automated tests use fake binaries that do not exercise real TUI rendering.
---

## Summary

`tessariq run --agent opencode --interactive` records `applied.interactive = false` and prints a warning that interactive is not natively supported. The runtime infrastructure for interactive mode (TTY allocation, activity-based timeout, docker attach session, direct container attach for `--attach`) is already agent-agnostic and functional. This task promotes opencode interactive from "recorded but not applied" to "fully applied."

## Acceptance Criteria

- `tessariq run --agent opencode --interactive` no longer prints the "not natively supported" warning.
- `agent.json` for an opencode interactive run records `applied.interactive = true`.
- OpenCode launches in TUI mode (no `run --format json` subcommand) when `--interactive` is set.
- The opencode TUI is functional inside the Docker container with TTY allocation (`-i -t`).
- `tessariq run --agent opencode --interactive --attach` lets the user interact with the opencode TUI via direct container attach.
- Non-interactive opencode behavior is unchanged (`run --format json` path).
- Claude Code interactive behavior is unchanged.

## Test Expectations

- Update 3 unit tests in `opencode_test.go` that assert `applied["interactive"] = false` to assert `true`.
- Update 2 factory tests in `factory_test.go` that assert opencode interactive applied = false.
- Update 2 e2e tests in `run_e2e_test.go` that assert opencode interactive evidence as not applied.
- No new test files needed — this is a flag flip with test assertion updates.

## TDD Plan

1. RED: update `TestBuildApplied_Interactive` in `opencode_test.go` to expect `applied["interactive"] = true`.
2. GREEN: change `buildApplied()` in `opencode.go` to return `"interactive": true`.
3. RED: update `TestBuildApplied_WithModel` and `TestBuildApplied_WithoutModel` to expect interactive = true.
4. GREEN: already passing from step 2.
5. RED: update factory tests for opencode interactive applied.
6. GREEN: already passing from step 2.
7. Update e2e tests to assert `applied["interactive"] = true` for opencode.
8. Remove the opencode interactive warning in `cmd/tessariq/run.go` (lines 90-93).
9. Update `buildArgs` comment to remove "not yet validated in tessariq."
10. Manual test: run `tessariq run --agent opencode --interactive --attach --egress open --image <image> <task>` and verify TUI interaction.

## Notes

- The `buildArgs()` function already handles the interactive split correctly — it omits `run --format json` when interactive, producing `opencode [--model M] -- <task>`.
- The comment on `buildArgs` line 95 says "TUI, not yet validated in tessariq" — update after manual validation.
- The runtime infrastructure (container TTY, activity timer, docker attach, session ready signaling) is agent-agnostic and requires no changes.
- If manual testing reveals that `opencode -- <task>` does not accept task content as initial prompt in TUI mode, `buildArgs()` will need adjustment to omit the task content in interactive mode. Document findings in the completion note.
- Likely files: `internal/adapter/opencode/opencode.go`, `internal/adapter/opencode/opencode_test.go`, `internal/adapter/factory_test.go`, `cmd/tessariq/run.go`, `cmd/tessariq/run_e2e_test.go`.
