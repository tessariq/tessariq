---
id: TASK-102-evidence-package-and-atomic-json
title: Introduce internal/evidence for shared schema validation and atomic JSON writes
status: blocked
priority: medium
dependencies:
    - TASK-017-v0-1-0-spec-conformity-closeout
milestone: v0.2.0
spec_version: v0.2.0
spec_ref: specs/tessariq-v0.2.0.md#evidence-additions
spec_refs:
    - specs/tessariq-v0.2.0.md#evidence-additions
    - specs/tessariq-v0.2.0.md#shared-runtime-sketch
updated_at: "2026-06-10T00:00:00Z"
areas:
    - evidence
    - runner
    - promote
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Schema validation, atomic write, and completeness helpers are pure logic and must be unit-tested directly.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Promote and runner completeness consume these helpers across package boundaries; the contract must hold end-to-end.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: No user-facing flow changes; covered transitively by existing run/promote e2e.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Completeness and schema-validation branches are fail-closed security logic and must survive mutation.
    manual_test:
        required: true
        commands: []
        rationale: Confirms evidence artifacts are byte-for-byte unchanged and promote still rejects the same tamper/incompleteness cases.
---

## Summary

Create an `internal/evidence` package that owns (1) shared artifact schema validation, (2) an atomic JSON write helper (mkdir + marshal + `.tmp` + rename with owner-only perms), and (3) the structured completeness checks currently performed generically in `internal/runner/completeness.go`. Migrate existing call sites to use it without changing any evidence artifact contents or promote behavior.

## Motivation

Evidence schemas are currently owned by separate packages, while `runner.CheckEvidenceCompleteness` validates some structured JSON generically with `map[string]any` to dodge import cycles. The tmp-file-plus-rename atomic-write pattern is independently re-implemented across several packages (`WriteManifest`, `WriteStatus`, and others). v0.2.0 adds new evidence artifacts (`specs/tessariq-v0.2.0.md#evidence-additions`), so schema evolution and write-pattern drift become real risks. A dedicated package removes the import-cycle pressure that forced `map[string]any` and gives one place to evolve artifact schemas.

This is review findings #4 (evidence schema ownership distributed) and #7 (atomic JSON writing repeated) addressed together, since #7 is only worth doing as part of #4.

## Acceptance Criteria

- A new `internal/evidence` package exposes an atomic JSON write helper that creates parent dirs with `0700`, writes via a `.tmp` file, fsync/rename into place, and sets `0600` on the final file — matching current evidence permission contracts.
- The package exposes typed schema validation for the structured artifacts currently checked in `completeness.go`, replacing generic `map[string]any` parsing where the concrete schema is known.
- `internal/runner/completeness.go` consumes the new validation helpers and no longer carries import-cycle workarounds for those artifacts. Fail-closed behavior (empty/missing/inconsistent egress evidence is rejected) is preserved exactly.
- Repeated atomic-write implementations in evidence-producing packages (manifest, status, runtime, workspace, agent, and proxy artifacts as applicable) are routed through the shared helper. No artifact bytes change.
- No import cycles are introduced; `internal/evidence` depends only on stdlib and small shared types.
- All existing completeness, promote, and runner tests pass with assertions unchanged.

## Non-Goals

- No new evidence artifacts (those are separate v0.2.0 tasks; this task makes adding them safe).
- No change to artifact field names, ordering, or JSON shape.
- No broad abstraction beyond the write helper, schema validators, and completeness checks.

## Test Expectations

- Unit tests for the atomic write helper: parent-dir `0700`, file `0600`, tmp+rename atomicity, and marshal failure leaving no partial file.
- Unit tests for the typed schema validators reproducing current `completeness.go` rejection cases (empty/missing/inconsistent egress evidence).
- Integration tests confirm `promote` still rejects the same tamper and incompleteness cases through the shared helpers.
- Mutation testing on the fail-closed completeness and validation branches.

## TDD Plan

- RED: unit test for the atomic write helper asserting parent-dir `0700`, file `0600`, and atomic rename semantics; fails because the package does not exist.
- GREEN: implement the helper; route one call site (manifest) through it.
- RED: unit test for typed schema validation of one structured artifact reproducing a current `completeness.go` rejection case.
- GREEN: implement validators; migrate `completeness.go`.
- REFACTOR: route remaining atomic-write call sites through the helper; delete the duplicated implementations.

## Notes

- Blocked until `v0.2.0` becomes the active milestone; depends on the v0.1.0 closeout (`TASK-017`).
- Review priority #3 ("Define a clearer evidence schema ownership model, likely via `internal/evidence`").
- Should land before the v0.2.0 evidence-additions feature tasks so new artifacts adopt the shared contract from the start. Independent of `TASK-101` but complementary.
