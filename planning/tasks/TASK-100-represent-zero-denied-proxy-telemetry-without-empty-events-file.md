---
id: TASK-100-represent-zero-denied-proxy-telemetry-without-empty-events-file
title: Represent zero-denied proxy telemetry without an empty events file
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#networking-and-egress
dependencies:
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-088-require-proxy-evidence-completeness-before-promote
    - TASK-096-make-proxy-telemetry-extraction-fail-closed
updated_at: "2026-04-24T09:17:04Z"
---

## Summary

A proxy-mode run whose access log contains zero denied destinations currently produces a zero-byte `egress.events.jsonl`, which `runner.CheckEvidenceCompleteness` rejects as incomplete. This makes honest proxy runs unpromotable and also fails to distinguish `no denied events occurred` from `the telemetry artifact is empty or synthetic`. Define a non-empty, parseable representation for the zero-denied-events case and keep it distinct from telemetry extraction failure.

## Supersedes

- BUG-064 from `planning/BUGS.md`.

## Acceptance Criteria

- A successful proxy-mode run with zero denied events emits a non-empty, parseable, trustworthy `egress.events.jsonl` representation.
- Completeness accepts that zero-denied-events representation as valid proxy evidence.
- Promote accepts a proxy-mode run with intact zero-denied-events evidence.
- The zero-denied-events case remains distinguishable from telemetry extraction failure; failure paths still fail closed per TASK-096.
- Existing proxy runs with one or more denied events continue to record event-per-line JSONL without behavior regression.

## Test Expectations

- Add unit coverage for writing and reading the zero-denied-events representation.
- Add completeness coverage proving that the zero-denied-events artifact passes while a genuinely empty artifact still fails.
- Add integration coverage exercising a real proxy run with no denied destinations and verifying promote-facing evidence is accepted.
- Add regression coverage that telemetry extraction failure still does not fabricate a clean result.
- Run mutation testing because the empty-file and zero-events branches are easy to blur.

## TDD Plan

1. RED: add failing tests showing that a no-denied-events proxy run currently produces an empty artifact that completeness rejects.
2. GREEN: choose and implement one non-empty parseable representation for zero denied events.
3. GREEN: keep telemetry extraction failure distinct and fail closed.
4. VERIFY: rerun unit, integration, mutation, and manual proxy checks.

## Notes

- Keep the representation minimal and easy to audit; the important contract is non-empty, parseable, and distinguishable from extraction failure.
- Coordinate this task with TASK-096 so the two proxy-telemetry cases are fixed together rather than reintroducing ambiguity.
- Add this task to `TASK-017-v0-1-0-spec-conformity-closeout` dependencies so closeout cannot pass while honest zero-denied proxy runs remain unpromotable.
- 2026-04-24T09:17:04Z: zero-denied proxy runs emit summary line; completeness requires non-empty; extraction failure stays distinct
