---
id: TASK-037-prestart-agent-binary-validation
title: Validate selected agent binary in runtime image before agent start
status: todo
priority: p0
depends_on:
    - TASK-024-claude-code-agent-runtime-integration
    - TASK-025-opencode-agent-runtime-integration
    - TASK-027-container-lifecycle-and-mount-isolation
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-31T20:30:00Z"
areas:
    - agents
    - runtime
    - failure-ux
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Missing-binary detection and error mapping should be deterministic and unit-testable.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Runtime-image checks execute against real containers and require integration coverage.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: This is a user-visible run failure mode promised by spec acceptance criteria.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Failure-path message quality and detection branching are easy to regress.
    manual_test:
        required: true
        commands: []
        rationale: Validate end-user error guidance wording and image override advice.
---

## Summary

The run path currently detects missing agent binaries reactively via container exit failures. This task adds explicit pre-start runtime validation so missing `claude` or `opencode` binaries fail before agent start with actionable guidance.

## Supersedes

- BUG-005 from `planning/BUGS.md`.

## Acceptance Criteria

- Before starting the agent command, Tessariq validates that the selected agent binary exists in the resolved runtime image.
- Missing binary failures occur before agent start and include: missing binary name, selected agent, and guidance to use a compatible runtime image or `--image` override.
- Validation behavior is implemented for both supported agents (`claude-code`, `opencode`).
- Existing successful run path for valid images remains unchanged.

## Test Expectations

- Add unit tests for missing-binary error classification and message content.
- Add integration tests for each supported agent with images that intentionally lack required binaries.
- Add e2e test(s) covering user-facing failure guidance for missing binaries.
- Ensure existing integration tests that relied on exit code 127 are updated to pre-start validation behavior.

## TDD Plan

1. RED: add failing tests for pre-start binary validation hooks for both agents.
2. RED: add failing tests asserting required failure UX message fields.
3. GREEN: implement runtime-image binary existence probe prior to launching agent process.
4. REFACTOR: keep validation reusable and agent-agnostic where practical.
5. GREEN: update integration/e2e tests to assert pre-start failure behavior.

## Notes

- Likely files: `internal/adapter/factory.go`, `internal/container/process.go` (or helper), `cmd/tessariq/run.go`, and agent integration/e2e tests.
- Avoid introducing host-tool dependencies; use containerized checks consistent with existing architecture.
