---
id: TASK-018-replace-yolo-with-interactive-and-cli-polish
title: Replace --yolo with --interactive and polish CLI flags
status: done
priority: p0
depends_on:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#adapter-contract
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#specification-changelog
updated_at: "2026-03-29T20:18:47Z"
areas:
    - cli
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Flag renaming, default changes, and custom duration formatting must be covered by unit tests.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: No integration-level behavior changes; this is purely CLI surface and config struct work.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: E2E coverage deferred until the run pipeline is executable end-to-end.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Default value changes and duration formatting logic are mutation-prone.
    manual_test:
        required: true
        commands: []
        rationale: Validates --help output reads correctly and flag behavior matches spec.
---

## Summary

Replace `--yolo` with `--interactive` (inverted semantics, autonomous-by-default), rename `--egress-allow-reset` to `--egress-no-defaults`, and fix inconsistent duration display in `--help` output.

## Acceptance Criteria

### --yolo -> --interactive

- The `--yolo` flag is removed from the CLI surface entirely.
- A new `--interactive` boolean flag exists with default `false`.
- Default behavior (no flag) means the agent runs autonomously inside the container sandbox without requiring human approval for tool use.
- `--interactive` opts in to human-in-the-loop approval mode, intended for use with `--attach`.
- The `Config` struct field is renamed from `Yolo bool` to `Interactive bool`.
- `DefaultConfig()` returns `Interactive: false` (autonomous by default).
- The `--help` description for `--interactive` communicates its purpose clearly, e.g. `"require human approval for agent tool use (use with --attach)"`.
- When `--interactive` is set without `--attach`, a warning is printed to stderr: the agent will block waiting for approval with no terminal attached.
- Existing unit tests for the old `Yolo` field are updated to test `Interactive` with inverted semantics.

### --egress-allow-reset -> --egress-no-defaults

- The `--egress-allow-reset` flag is removed from the CLI surface entirely.
- A new `--egress-no-defaults` boolean flag exists with default `false`.
- The `Config` struct field is renamed from `EgressAllowReset bool` to `EgressNoDefaults bool`.
- The `--help` description reads `"ignore default allowlists; only --egress-allow entries apply"`.
- Existing validation and unit tests are updated for the renamed field.

### Duration display fix

- A custom `pflag.Value` type wraps `time.Duration` with a `String()` that strips trailing zero components (e.g. `30m0s` -> `30m`, `1h0m0s` -> `1h`).
- Both `--timeout` and `--grace` use the custom duration type.
- Parsing still accepts standard Go duration strings via `time.ParseDuration`.
- The `--help` output displays `(default 30m)` for timeout and `(default 30s)` for grace, with consistent formatting.

## Test Expectations

- Update all existing unit tests that reference `Yolo` or `EgressAllowReset` to use the new field names and semantics.
- Add unit tests for the custom duration type: formatting edge cases (`30m0s` -> `30m`, `1h0m0s` -> `1h`, `1h30m0s` -> `1h30m`, `5m30s` -> `5m30s`, `90s` -> `1m30s`, `500ms` -> `500ms`), and parsing round-trips.
- Add a unit test verifying `DefaultConfig()` returns `Interactive: false` and `EgressNoDefaults: false`.
- Run mutation testing because default value changes and formatting logic are branch-prone.

## TDD Plan

- Start with failing unit tests for `DefaultConfig()` returning the new field names and defaults, then implement the renames.
- Then add failing tests for the custom duration `String()` method, then implement the type.

## Notes

- This task is safe to execute immediately after TASK-002 since it only modifies Config struct fields, flag definitions, and help text.
- Adapter tasks (TASK-008, TASK-009, TASK-010) are not yet implemented, so the rename has zero migration cost.
- The spec changelog entry for 2026-03-29 documents the rationale for these changes.
- 2026-03-29T20:18:47Z: Renamed Yolo→Interactive, EgressAllowReset→EgressNoDefaults in Config struct and CLI flags. Added custom DurationValue pflag.Value type for clean --help output (30m not 30m0s). Added --interactive without --attach stderr warning. All tests pass, mutation efficacy 97.30%. Evidence: planning/artifacts/manual-test/TASK-018-replace-yolo-with-interactive-and-cli-polish/20260329T201752Z/report.md, planning/artifacts/verify/task/TASK-018-replace-yolo-with-interactive-and-cli-polish/20260329T201839Z/report.json
