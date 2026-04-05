---
id: TASK-079-forward-model-flag-and-adapter-interface
title: Forward --model flag to OpenCode and introduce adapter Agent interface
status: done
priority: p2
depends_on:
    - TASK-010-opencode-adapter
    - TASK-008-adapter-contract-and-adapter-json
milestone: v0.2.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
updated_at: "2026-04-05T20:41:35Z"
areas:
    - adapter
    - cli
verification:
    unit:
        required: true
        commands:
            - go test ./internal/adapter/...
            - go test ./...
        rationale: Adapter arg building and interface conformance need precise unit coverage.
    integration:
        required: false
        commands: []
        rationale: No new subsystem boundaries introduced; factory dispatch is covered by unit tests.
    e2e:
        required: false
        commands: []
        rationale: Model forwarding is a CLI arg change, not a runtime flow change.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Conditional branching on model presence is mutation-sensitive.
    manual_test:
        required: false
        commands: []
        rationale: Model forwarding can be fully validated through unit tests on adapter arg building.
---

## Summary

`tessariq run --agent opencode --model X` accepts the `--model` flag but silently drops it. The opencode adapter records `applied["model"] = false`. Meanwhile, the claude-code adapter forwards `--model` as-is and records `applied["model"] = true`. The original rationale was a format mismatch (tessariq shorthand vs opencode's `provider/model` format), but tessariq does not define or validate model strings — it is a passthrough. Users should pass the agent-native format directly.

Additionally, the factory in `factory.go` duplicates 7-field extraction across both switch arms, revealing a missing interface.

## Acceptance Criteria

- `tessariq run --agent opencode --model anthropic/claude-sonnet-4-20250514 <task>` forwards `--model anthropic/claude-sonnet-4-20250514` to the opencode CLI.
- `agent.json` records `applied.model = true` when `--model` is provided for opencode runs.
- A formal `Agent` interface is defined in `internal/adapter/` with the 7 methods both adapters already implement: `Name()`, `BinaryName()`, `Args()`, `Image()`, `Requested()`, `Applied()`, `EnvVars()`.
- Both `claudecode.AgentConfig` and `opencode.AgentConfig` satisfy the `Agent` interface.
- The factory switch in `NewProcess` is simplified to use the interface, eliminating duplicated field extraction.
- Existing behavior for claude-code is unchanged.
- Interactive mode behavior for opencode is unchanged (model forwarded, interactive still `applied: false`).

## Test Expectations

- Unit tests for opencode adapter verify `--model <value>` appears in args when model is set.
- Unit tests for opencode adapter verify `applied["model"] = true` when model is set.
- Unit tests for `NewAgent` factory verify correct dispatch and error on unknown agent.
- Unit tests for `Name()` and `BinaryName()` methods on both adapters.
- Existing claude-code adapter tests remain green without changes.
- Factory tests updated to expect `applied["model"] = true` for opencode with model.

## TDD Plan

1. RED: add interface conformance compile-time checks for both adapters.
2. GREEN: define `Agent` interface, add `Name()` and `BinaryName()` methods to both adapters.
3. RED: update opencode `TestBuildArgs_WithModel` to expect `--model` in args.
4. GREEN: forward `--model` in opencode `buildArgs`.
5. RED: update opencode `TestBuildApplied_WithModel` to expect `true`.
6. GREEN: change `buildApplied` to return `true` for model.
7. RED: add `TestNewAgent` tests for factory dispatch.
8. GREEN: implement `NewAgent` and simplify `NewProcess`.

## Notes

- The `Agent` interface cannot be referenced from the subpackages (`claudecode`, `opencode`) due to circular imports. The compile-time check uses an anonymous interface literal instead. This is idiomatic Go — interfaces are defined where consumed, not where implemented.
- Tessariq remains format-agnostic for the model string. For Claude Code, users pass shorthands (e.g. `sonnet`). For OpenCode, users pass `provider/model` format (e.g. `anthropic/claude-sonnet-4-20250514`). Each agent interprets its own format.
- 2026-04-05T20:41:35Z: Forwarded --model to opencode CLI as-is, introduced adapter.Agent interface, simplified factory dispatch. Tests: all pass, mutation efficacy 85.74%. Manual tests: 4/4 pass.
