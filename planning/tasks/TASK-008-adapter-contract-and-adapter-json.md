---
id: TASK-008-adapter-contract-and-adapter-json
title: Implement adapter contract and adapter metadata recording
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#adapter-contract
dependencies:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
updated_at: "2026-03-30T15:34:54Z"
---

## Summary

Create shared adapter abstractions and `adapter.json` emission rules for v0.1.0.

## Acceptance Criteria

- `adapter.json` uses the v0.1.0 minimum shape with `schema_version`, `adapter`, `image`, `requested`, and `applied`.
- `adapter.json` always records requested options, including options that later prove unsupported.
- Unsupported exact application is recorded explicitly in `applied` without erasing the original requested values.
- Concrete adapters can plug into the shared contract without changing schema version 1.

## Test Expectations

- Add unit tests for `adapter.json` shaping and requested/applied semantics.
- Integration tests are deferred until adapters run real processes through the run lifecycle.
- E2E tests are deferred until adapter-specific tasks land.
- Run mutation testing because adapter bookkeeping is logic-heavy enough to justify it.

## TDD Plan

- Start with a failing unit test for `adapter.json` requested/applied behavior.

## Notes

- Keep schema_version stable at `1` for v0.1.0 artifacts.
- 2026-03-30T15:34:54Z: adapter contract implemented in internal/adapter/info.go; 8 unit tests, 91.25% mutation efficacy, 0 verification findings; local-only manual-test artifacts generated.
