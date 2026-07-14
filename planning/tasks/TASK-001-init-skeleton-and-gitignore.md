---
id: TASK-001-init-skeleton-and-gitignore
title: Initialize repository skeleton and ignore generated runtime state
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#release-intent
dependencies: []
updated_at: "2026-03-29T13:39:40Z"
---

## Summary

Create `tessariq init` behavior for the repo skeleton and `.gitignore` update.

## Acceptance Criteria

- `.tessariq/runs/` is created when missing at the repository root.
- `.tessariq/` is added to `.gitignore` without duplicating entries, creating or updating `.gitignore` as needed.
- The command behaves cleanly on reruns.
- The task continues to treat `.tessariq/` as repo-local generated state, not repo-tracked config.

## Test Expectations

- Add or update unit tests for idempotent directory and ignore-file handling.
- Integration tests are not needed unless filesystem orchestration becomes multi-step enough to justify a containerized boundary.
- E2E tests are not needed yet because the broader CLI workflow is not exercised here.
- Run mutation testing because idempotent `.gitignore` handling has meaningful branch coverage.

## TDD Plan

- Start with a failing unit test for idempotent `.gitignore` insertion and required directory creation.

## Notes

- Keep the implementation behavior-preserving outside the new `init` command.
- 2026-03-29T13:39:40Z: Unit tests pass (8 scenarios), mutation efficacy 76.92% (>70% threshold), manual test 6/6 pass. Local-only manual-test and verification artifacts generated.
