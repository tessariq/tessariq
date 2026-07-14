---
id: TASK-009-claude-code-adapter
title: Implement the Claude Code adapter
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#adapter-contract
dependencies:
    - TASK-008-adapter-contract-and-adapter-json
updated_at: "2026-03-30T15:58:12Z"
---

## Summary

Implement the first-party `claude-code` adapter on top of the shared adapter contract.

## Acceptance Criteria

- `adapter.json` records `adapter=claude-code` and the resolved image value used for the run.
- Requested adapter options are forwarded when supported.
- Unsupported exact application is recorded in `adapter.json`, including partial application of `--model` and `--interactive`.
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
- 2026-03-30T15:58:12Z: Claude Code adapter implemented. Evidence: unit tests (internal/adapter/claudecode/claudecode_test.go), integration tests (claudecode_integration_test.go), e2e test (cmd/tessariq/run_e2e_test.go), factory (internal/adapter/factory.go), wired into cmd/tessariq/run.go. Mutation testing 96.43% efficacy. Manual test completed with local-only artifacts. Verification completed with 0 findings and local-only artifacts.
