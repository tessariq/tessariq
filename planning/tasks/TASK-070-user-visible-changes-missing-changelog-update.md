---
id: TASK-070-user-visible-changes-missing-changelog-update
title: user-visible changes missing changelog update
status: done
priority: p1
depends_on: []
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#failure-ux
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: "2026-04-03T10:27:46Z"
areas:
    - workflow
    - verification
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Follow-up items start by adding the smallest failing unit test possible.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: Add only if the follow-up crosses a real process boundary and use Testcontainers only.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: Add only if the fix changes a critical CLI workflow end to end.
    mutation:
        required: false
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Use when the follow-up changes non-trivial logic.
    manual_test:
        required: false
        commands: []
        rationale: Run manual checks only if verification cannot confirm the user-visible changelog update deterministically.
---

## Summary

Address verification finding `TASK-068-make-manifest-writes-atomic-changelog`.

## Acceptance Criteria

- Finding is resolved or explicitly downgraded with evidence.

## Test Expectations

- Re-evaluate unit, integration, e2e, and mutation test needs before implementation.

## TDD Plan

- Start with the smallest failing test that reproduces the finding.

## Notes

- Source report finding: `User-visible code changes detected (internal/run/manifest.go) without updating CHANGELOG.md. Add a user-facing entry under CHANGELOG.md before finishing the task.`
- 2026-04-03T10:27:46Z: Added CHANGELOG.md entry for TASK-068 atomic manifest writes; removed stale merge conflict marker. Verification passes with zero findings.
