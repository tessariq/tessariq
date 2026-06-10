---
id: TASK-103-split-workflow-service
title: Split internal/workflow/service.go by concern
status: blocked
priority: p2
depends_on:
    - TASK-017-v0-1-0-spec-conformity-closeout
milestone: v0.2.0
spec_version: v0.2.0
spec_refs:
    - docs/workflow/autonomous-contract.md#verification-contract
    - docs/workflow/development-workflow.md#tdd-default
updated_at: "2026-06-10T00:00:00Z"
areas:
    - workflow
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: The workflow service is pure logic with extensive existing unit coverage; the split must keep every test green unchanged.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: No cross-process boundaries change.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: Internal tracked-work tooling, not part of the product CLI e2e flows.
    mutation:
        required: false
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Pure file reorganization with no logic change; mutation score is unaffected by moving functions between files.
    manual_test:
        required: true
        commands: []
        rationale: Confirms tessariq-workflow subcommands (validate-state, next, start, finish, verify, check-skills) behave identically after the split.
---

## Summary

Split `internal/workflow/service.go` (1418 lines, 58 functions) into several focused files within the same `workflow` package, grouped by concern. No behavior change, no exported-API change.

## Motivation

The workflow service is the single largest file in the repository and owns state transitions, task load/save, validation, verification-report construction and rendering, follow-up creation, skill checks, and spec coverage all in one place. It is internal tooling so user risk is low, but it is the clearest maintainability smell in the codebase and it will keep growing as tracked-work tooling expands. The functions are already well-decomposed (avg ~24 lines), so the fix is a mechanical, low-risk file split.

This is review finding #3 and review priority #2.

## Acceptance Criteria

- `service.go` is split into cohesive same-package files. Suggested grouping (adjust to natural seams found in code):
  - `service.go` — `Service` type, constructor, and the public command entrypoints.
  - `selection.go` — `Next`, `selectNextTask`, `eligibleTasks`, `candidateTasks`, `unresolvedDependency`, priority/severity ranking.
  - `validation.go` — `ValidateState`, `validateStateAndTasks`, section/rationale checks.
  - `verification.go` — `Verify`, report construction, findings builders, changelog/spec/implemented findings, summary/status.
  - `followups.go` — `CreateFollowups`, `newFollowupTask`, task-number derivation.
  - `persistence.go` — load/save state and tasks, rendering snapshots, skill-tree helpers.
- No function signatures change; the package's exported surface is identical.
- Every existing `internal/workflow` test passes without modification.
- `go vet ./...` and `gofmt -l .` stay clean. No file exceeds the 800-line guideline after the split.

## Non-Goals

- No logic changes, no renames of exported symbols, no new abstractions or interfaces.
- No change to the tracked-work state file format or `tessariq-workflow` CLI output.

## Test Expectations

- The full existing `internal/workflow` unit suite passes unchanged before and after the split.
- `go build ./...`, `go vet ./...`, and `gofmt -l .` stay clean after each file move.
- No new tests required; the split is behavior-preserving and covered by the existing suite.

## TDD Plan

- This is a refactor under existing test cover. Run the full `internal/workflow` suite before and after; the suite is the safety net.
- Move functions file-by-file, running `go build ./... && go test ./internal/workflow` after each move to catch accidental unexported-symbol breakage early.

## Notes

- Blocked until `v0.2.0` becomes the active milestone; depends on the v0.1.0 closeout (`TASK-017`).
- Lowest-risk item in the structural-cleanup set; good candidate to land first to build momentum.
