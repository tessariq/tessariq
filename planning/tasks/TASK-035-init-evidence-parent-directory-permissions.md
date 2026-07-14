---
id: TASK-035-init-evidence-parent-directory-permissions
title: Harden init-created evidence parent directories to owner-only permissions
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#evidence-permissions
dependencies:
    - TASK-001-init-skeleton-and-gitignore
    - TASK-032-container-security-hardening
updated_at: "2026-04-01T12:05:06Z"
---

## Summary

`init` currently creates `.tessariq/runs` with `0o755`, which is broader than the evidence permission contract. This task aligns init-created parent evidence directories with owner-only access requirements.

## Supersedes

- BUG-003 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq init` creates `.tessariq/` and `.tessariq/runs/` with owner-only directory permissions.
- Re-running `tessariq init` is idempotent and does not relax secure permissions.
- Existing `.gitignore` behavior for `.tessariq/` remains unchanged.
- Existing run-time evidence file permissions remain unchanged (`0o600` files, `0o700` run directories).

## Test Expectations

- Add unit tests asserting init-created directories are owner-only.
- Add regression test for idempotent re-run preserving secure permissions.
- Confirm no regressions in existing init tests.

## TDD Plan

1. RED: add failing unit test for `.tessariq/runs` mode after `initialize.Run`.
2. RED: add failing idempotency test that re-runs init and checks mode remains owner-only.
3. GREEN: update init directory mode(s) and, if needed, enforce chmod on existing dirs.
4. REFACTOR: keep permission handling minimal and explicit.
5. GREEN: run full unit suite.

## Notes

- Likely files: `internal/initialize/initialize.go` and `internal/initialize/initialize_test.go`.
- Be explicit about platform mode-mask interactions in tests (normalize with `Perm()` checks).
- 2026-04-01T12:05:06Z: Changed init directory permissions from 0o755 to 0o700 with idempotent chmod. Added 3 unit tests (permissions, idempotency, tightening). All tests pass, mutation efficacy 85%. Manual tests 4/4 pass.
