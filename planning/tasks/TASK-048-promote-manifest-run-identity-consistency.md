---
id: TASK-048-promote-manifest-run-identity-consistency
title: Verify promote evidence identity matches the resolved run
status: todo
priority: p0
depends_on:
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-047-promote-repo-local-evidence-path-validation
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#core-workflow
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-04-01T20:03:47Z"
areas:
    - promote
    - evidence
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Manifest-vs-run identity checks are deterministic and should begin with unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Promote side effects must be validated against tampered evidence in a real git repo.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: A forged manifest must be rejected on the user-facing promote path.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Identity cross-checks are safety-critical and branch-heavy.
    manual_test:
        required: true
        commands: []
        rationale: Confirms tampered manifest metadata cannot rewrite branch identity or commit trailers.
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
