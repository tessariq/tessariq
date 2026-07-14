---
id: TASK-014-run-index-and-run-ref-resolution
title: Maintain the run index and resolve repository-scoped run refs
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#lifecycle-rules
dependencies:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-013-diff-log-and-evidence-artifacts
    - TASK-022-agent-and-runtime-evidence-migration
updated_at: "2026-04-01T10:00:00Z"
---

## Summary

Append run index entries and implement repository-scoped run-ref resolution.

## Acceptance Criteria

- `index.jsonl` is append-only.
- `index.jsonl` entries include the minimum required fields for `run_id`, `created_at`, `task_path`, `task_title`, `agent`, `workspace_mode`, `state`, and `evidence_path`.
- `run_id`, `last`, and `last-N` resolve against the current repository's run index only.
- Commands fail cleanly when the referenced run cannot be found in the current repository.
- Shared run-ref resolution is the source of truth for attach and promote.

## Test Expectations

- Add unit tests for index entry rendering and run-ref resolution.
- Add unit tests for edge cases: `last` on empty index, `last-0` behavior, malformed JSON lines in index (graceful skip or explicit failure), and ULID format validation of index entries.
- Add integration tests for concurrent append safety (two runs finishing simultaneously) and corrupted-index recovery.
- Add thin e2e coverage because run refs feed user-facing attach and promote commands.
- Run mutation testing because ordering and resolution logic are subtle.

## TDD Plan

- Start with a failing unit test for `last-N` resolution and append-only index behavior.

## Notes

- Do not rewrite index entries in place for another run.
