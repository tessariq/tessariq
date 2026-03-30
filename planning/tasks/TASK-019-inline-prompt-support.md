---
id: TASK-019-inline-prompt-support
title: Support inline text prompts via --prompt flag
status: blocked
priority: p2
depends_on: []
milestone: v0.2.0
spec_version: v0.2.0
spec_refs:
    - specs/tessariq-v0.2.0.md#release-intent
    - specs/tessariq-v0.2.0.md#scope
    - specs/tessariq-v0.2.0.md#acceptance-scenarios
updated_at: "2026-03-30T22:28:00Z"
areas:
    - cli
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Prompt-vs-taskpath mutual exclusivity and synthesized task.md content should be unit-tested.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: Deferred until integrated into the full run pipeline.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: Deferred until the feature is part of a releasable workflow.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Mutual exclusivity and input synthesis logic are branchy.
    manual_test:
        required: true
        commands: []
        rationale: Validates the inline prompt UX end-to-end.
---

## Summary

Add a `--prompt` flag to `tessariq run` that accepts inline text instead of a task file path, enabling quick one-off agent runs without creating a markdown file first.

## Acceptance Criteria

- A new `--prompt <string>` flag exists on `tessariq run`.
- `--prompt` and positional `<task-path>` are mutually exclusive; providing both fails with clear guidance.
- When `--prompt` is used, tessariq synthesizes a `task.md` in the evidence directory containing the prompt text as a markdown H1 heading followed by the full text.
- `task_path` in `manifest.json` is recorded as `<inline>` when `--prompt` is used.
- `task_title` is derived from the first 80 characters of the prompt text.
- The dirty-repo check still applies (the container sandbox guarantee depends on a clean repo, not on where the task text comes from).
- `Args` validation changes from `ExactArgs(1)` to a custom validator: exactly 1 positional arg XOR `--prompt` provided.

## Test Expectations

- Unit tests for mutual exclusivity between `--prompt` and positional arg.
- Unit tests for synthesized `task.md` content and `task_title` derivation.
- Unit tests for `manifest.json` recording `task_path: "<inline>"`.
- Mutation testing for input routing logic.

## TDD Plan

- Start with a failing unit test for the mutual exclusivity validation.

## Notes

- Deferred to v0.2.0. The v0.1.0 thesis is file-based, git-tracked, reproducible tasks. Inline prompts should only be added after v0.1.0 validates that workflow and user feedback shows task-file creation is a barrier.
- This task is intentionally blocked until `v0.2.0` becomes the active milestone and its draft spec adds explicit `--prompt` contract language.
- Practical v0.1.0 workaround: `echo "# Fix the bug" > tasks/quick.md && tessariq run tasks/quick.md`.
- Spec changes for v0.2.0 will need to define `<inline>` semantics for `task_path` and document the reproducibility trade-off (inline prompts are ephemeral, not version-controlled).
