---
id: TASK-008-adapter-contract-and-adapter-json
title: Implement adapter contract and adapter metadata recording
status: todo
priority: p1
depends_on:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#adapter-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-03-29T12:06:20Z"
areas:
    - adapters
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Adapter option recording and applied/requested semantics should start with unit tests.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: Containerized integration tests can wait until concrete adapters are wired into run execution.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: End-to-end coverage belongs with concrete adapter flows rather than the shared contract alone.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Requested-versus-applied bookkeeping is a good mutation target.
    manual_test:
        required: true
        commands: []
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
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
