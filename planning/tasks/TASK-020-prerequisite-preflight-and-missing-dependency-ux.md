---
id: TASK-020-prerequisite-preflight-and-missing-dependency-ux
title: Add prerequisite preflight and missing dependency guidance
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#host-prerequisites
dependencies:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-003-dirty-repo-gate-and-task-ingest
    - TASK-006-tmux-session-and-detached-attach-guidance
updated_at: "2026-03-30T17:55:09Z"
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
- 2026-03-30T17:55:09Z: Implemented shared prerequisite preflight for init/run/attach mapping; init and run now fail fast with actionable missing-dependency guidance. Tests: go test ./..., go test -tags=e2e ./..., gremlins efficacy 96.97%. Local-only manual-test and verification artifacts generated.
