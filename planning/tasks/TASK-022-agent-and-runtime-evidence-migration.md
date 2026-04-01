---
id: TASK-022-agent-and-runtime-evidence-migration
title: Replace adapter evidence with agent and runtime evidence
status: done
priority: p0
depends_on:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-008-adapter-contract-and-adapter-json
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#adapter-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: "2026-03-30T22:06:40Z"
areas:
    - agents
    - runtime
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Evidence shaping and schema compatibility should start with unit tests.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: Integration coverage can remain with agent-specific run tasks.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: End-to-end coverage is better owned by full run-path tasks.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Requested-versus-applied bookkeeping and evidence migration are mutation-prone.
    manual_test:
        required: true
        commands: []
        rationale: Validates the visible evidence layout and JSON artifacts.
---

## Summary

Replace the superseded adapter-centric evidence model with `agent.json`, `runtime.json`, and `agent` fields in the manifest and run index.

## Acceptance Criteria

- `agent.json` replaces `adapter.json` as the requested/applied option artifact.
- `runtime.json` records runtime-image and mount-policy metadata.
- `manifest.json` uses `agent`, not `adapter`.
- `index.jsonl` uses `agent`, not `adapter`.
- The active v0.1.0 schema contract remains `schema_version: 1` for all required JSON artifacts.
- Historical completed-task references remain valid through the compatibility note in the active spec; they do not block the new runtime evidence model.

## Test Expectations

- Add unit tests for `agent.json` shaping and requested/applied semantics.
- Add unit tests for `runtime.json` shaping and mount-policy recording.
- Add unit tests for `manifest.json` and `index.jsonl` using `agent` fields.
- Run mutation testing because the evidence migration is logic-heavy enough to justify it.

## TDD Plan

- Start with a failing unit test for `agent.json` and `runtime.json` minimum-shape behavior.

## Notes

- This task supersedes the old adapter-evidence model without rewriting the historical done task that introduced it.
- 2026-03-30T22:06:40Z: Replaced adapter.json with agent.json + runtime.json per v0.1.0 spec. Manifest uses agent field with resolved_egress_mode and allowlist_source. All unit tests pass, e2e tests updated. Local-only verification and manual-test artifacts generated.
