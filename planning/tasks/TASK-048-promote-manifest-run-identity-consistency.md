---
id: TASK-048-promote-manifest-run-identity-consistency
title: Verify promote evidence identity matches the resolved run
status: completed
priority: high
spec_ref: specs/tessariq-v0.1.0.md#core-workflow
dependencies:
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-047-promote-repo-local-evidence-path-validation
updated_at: "2026-04-02T13:54:54Z"
---

## Summary

After resolving a run, `promote` currently trusts `manifest.json` fields for branch naming and commit trailers without verifying they belong to that run. Add consistency checks so the resolved run reference, evidence directory, and manifest identity cannot diverge.

## Supersedes

- BUG-015 from `planning/BUGS.md`.

## Acceptance Criteria

- `promote <run-ref>` fails if `manifest.json.run_id` does not match the resolved run ID and evidence directory name.
- Branch defaults, fallback commit messages, and default trailers are derived only from validated same-run metadata.
- Manifest tampering cannot cause `promote RUN_A` to create a branch or commit claiming a different run.
- Failure messaging identifies inconsistent or tampered evidence rather than creating git side effects.

## Test Expectations

- Add unit tests for manifest run-id mismatch and evidence-directory mismatch handling.
- Add integration or e2e regression showing a tampered manifest cannot change branch naming or trailer values.
- Add regression coverage that valid manifests continue to produce the existing branch and trailer behavior.

## TDD Plan

1. RED: add a failing test where the manifest run ID differs from the resolved run.
2. GREEN: enforce run-identity consistency before branch-name and message construction.
3. REFACTOR: centralize evidence-integrity checks ahead of patch application.
4. GREEN: verify no branch or commit is created on mismatch.

## Notes

- Likely files: `internal/promote/promote.go` and promote integration/e2e tests.
- Keep branch naming and trailer formats unchanged for valid runs.
- 2026-04-02T13:54:54Z: Added manifest identity validation in promote flow: manifest.RunID must match both the resolved IndexEntry.RunID and the evidence directory name. Tampered manifests are rejected before any git side effects. Unit, integration, e2e, mutation, and manual tests all pass.
