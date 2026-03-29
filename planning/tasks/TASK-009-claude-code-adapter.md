---
id: TASK-009-claude-code-adapter
title: Implement the Claude Code adapter
status: todo
priority: p1
depends_on:
    - TASK-008-adapter-contract-and-adapter-json
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#adapter-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-03-29T12:06:20Z"
areas:
    - adapters
    - claude-code
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Adapter command construction and option application should start with unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Real adapter invocation touches process boundaries and should use Testcontainers-backed integration coverage only.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Adapter behavior affects the end-to-end user flow and deserves thin CLI coverage.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Option mapping and partial-application reporting are branch-heavy.
    manual_test:
        required: true
        commands: []
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
---

## Summary

Implement the first-party `claude-code` adapter on top of the shared adapter contract.

## Acceptance Criteria

- `adapter.json` records `adapter=claude-code` and the resolved image value used for the run.
- Requested adapter options are forwarded when supported.
- Unsupported exact application is recorded in `adapter.json`, including partial application of `--model` and `--yolo`.
- The adapter integrates cleanly with the run lifecycle.

## Test Expectations

- Add unit tests for command/option translation.
- Add integration tests for adapter process invocation using Testcontainers-backed collaborators only.
- Add integration tests for adapter binary not-found error handling (clean failure with user guidance when `claude` is absent from the container image).
- Add integration tests for adapter process crash mid-run (unexpected exit code, no output).
- Add a thin e2e run path once the adapter is wired into run execution.
- Run mutation testing because option translation is branchy.

## TDD Plan

- Start with a failing unit test for Claude Code option translation and `adapter.json` emission.

## Notes

- Preserve the evidence contract even when the adapter cannot apply an option exactly.
