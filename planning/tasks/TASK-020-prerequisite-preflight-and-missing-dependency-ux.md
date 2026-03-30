---
id: TASK-020-prerequisite-preflight-and-missing-dependency-ux
title: Add prerequisite preflight and missing dependency guidance
status: todo
priority: p0
depends_on:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-003-dirty-repo-gate-and-task-ingest
    - TASK-006-tmux-session-and-detached-attach-guidance
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#host-prerequisites
    - specs/tessariq-v0.1.0.md#tessariq-init
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#tessariq-attach-run-ref
    - specs/tessariq-v0.1.0.md#failure-ux
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: "2026-03-30T20:35:00Z"
areas:
    - cli
    - ux
    - prerequisites
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Prerequisite detection and error-message shaping are branch-heavy and should start with unit coverage.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: Integration tests are optional unless this task adds process collaborators beyond existing command execution paths.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Missing-prerequisite behavior is directly user-visible CLI UX and needs thin end-to-end validation.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Preflight decision branches and failure guidance should survive mutation testing.
    manual_test:
        required: true
        commands: []
        rationale: Validates actionable prerequisite failures in real CLI workflows.
---

## Summary

Add shared prerequisite preflight handling so missing dependencies fail fast with consistent, actionable error guidance.

## Acceptance Criteria

- `tessariq init` fails cleanly when `git` is missing or unavailable and tells the user how to recover.
- `tessariq run` fails cleanly before run lifecycle side effects when required local prerequisites are missing or unavailable (`git`, `tmux`, and the runtime dependency checks owned by this task's scope).
- `tessariq attach` fails cleanly with actionable guidance when `tmux` is missing or unavailable.
- Prerequisite failures identify the missing dependency by name and include install/enable-and-retry guidance.
- Failure paths do not print success-formatted detached guidance (`run_id`, `attach`, `promote`) when preflight fails.

## Test Expectations

- Add unit tests for prerequisite detection outcomes and normalized user-facing error text.
- Add unit tests for command-specific prerequisite mapping (`init`, `run`, `attach`).
- Add error-path e2e tests covering missing `git` and missing `tmux` scenarios with actionable output checks.
- Run mutation testing for preflight and error-guidance branching.

## TDD Plan

- Start with a failing unit test for missing `git` guidance, then add failing tests for missing `tmux` handling in run and attach paths.

## Notes

- Keep error messages script-friendly and actionable.
- Docker runtime preflight behavior for proxy/container topology remains coordinated with `TASK-012`.
