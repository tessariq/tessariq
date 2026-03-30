---
id: TASK-010-opencode-adapter
title: Implement the OpenCode adapter
status: done
priority: p1
depends_on:
    - TASK-008-adapter-contract-and-adapter-json
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#adapter-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-30T18:41:18Z"
areas:
    - adapters
    - opencode
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

Implement the first-party `opencode` adapter on top of the shared adapter contract.

## Acceptance Criteria

- `adapter.json` records `adapter=opencode` and the resolved image value used for the run.
- Requested adapter options are forwarded when supported.
- Unsupported exact application is recorded in `adapter.json`, including partial application of `--model` and `--interactive`.
- The adapter integrates cleanly with the run lifecycle.
- Missing adapter binary failures include actionable user guidance that names `opencode` and indicates the required container image/runtime expectation.

## Test Expectations

- Add unit tests for command/option translation.
- Add integration tests for adapter process invocation using Testcontainers-backed collaborators only.
- Add integration tests for adapter binary not-found error handling (clean failure with user guidance when `opencode` is absent from the container image).
- Add unit tests for user-facing binary-not-found error message formatting consistency across adapters.
- Add integration tests for adapter process crash mid-run (unexpected exit code, no output).
- Add a thin e2e run path once the adapter is wired into run execution.
- Run mutation testing because option translation is branchy.

## TDD Plan

- Start with a failing unit test for OpenCode option translation and `adapter.json` emission.

## Notes

- Preserve the evidence contract even when the adapter cannot apply an option exactly.
- 2026-03-30T18:41:18Z: Implemented opencode adapter with unit tests (18), integration tests (4), e2e tests (2), manual test (1). Binary-not-found wrapping added to both adapters for consistency. Mutation efficacy 94.94%. Evidence: planning/artifacts/verify/task/TASK-010-opencode-adapter/20260330T183958Z/, planning/artifacts/manual-test/TASK-010-opencode-adapter/20260330T204000Z/
