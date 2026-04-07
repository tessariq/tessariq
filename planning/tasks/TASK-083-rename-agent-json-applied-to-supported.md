---
id: TASK-083-rename-agent-json-applied-to-supported
title: Rename agent.json applied field to supported
status: done
priority: p0
depends_on:
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-082-clarify-applied-field-semantics
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#compatibility-rules
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-04-07T17:57:10Z"
areas:
    - adapters
    - evidence
    - spec
    - docs
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: The rename changes shared adapter metadata types, JSON shaping, and evidence assertions across multiple packages.
    integration:
        required: false
        commands: []
        rationale: No new subsystem boundary is introduced; the change is primarily a contract and evidence-shape rename.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: End-to-end tests must prove emitted agent.json artifacts use supported and no longer emit applied.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: The requested-versus-supported bookkeeping is simple, branchy, and easy to regress with superficial renames.
    manual_test:
        required: true
        commands: []
        rationale: A real run should be inspected to confirm agent.json uses supported, omits applied, and matches the updated contract.
---

## Summary

`agent.json.applied` now means "can this agent honor this recorded option exactly?" rather than "was this option applied during this run". Rename the field to `supported` so the evidence contract matches the actual semantics, and remove `applied` outright instead of carrying a deprecated alias.

This is an intentional breaking evidence-contract change within the v0.1.0 line. The task must update the spec, implementation, tests, docs, and closeout gating together so the repository has one consistent contract.

## Acceptance Criteria

- The v0.1.0 spec uses `supported` as the canonical field name in the agent/runtime contract and evidence contract.
- The minimum `agent.json` shape in the spec uses `supported` and does not mention `applied`.
- The implementation emits `agent.json.supported` and no longer emits `agent.json.applied`.
- `internal/adapter.AgentInfo` uses `Supported map[string]bool` with `json:"supported"`.
- Adapter-facing APIs and helpers are renamed consistently, including `Supported()` and `buildSupported(...)`.
- Existing capability semantics are preserved exactly; only the field name changes.
- `agent.json` continues to use `schema_version: 1`.
- The evidence compatibility rules in the active spec are updated so the rename is explicit and no longer describe the old field as required.
- README and changelog language refer to requested versus supported agent options rather than requested versus applied options.
- E2e evidence assertions validate `supported` and confirm `applied` is absent.
- Unit tests for agent metadata shape assert `supported` is present and `applied` is absent.
- `TASK-017-v0-1-0-spec-conformity-closeout` depends on this task so final closeout cannot run before the rename lands.

## Test Expectations

- Add or update unit tests in `internal/adapter/agent_test.go` to assert JSON output contains `supported` and omits `applied`.
- Update adapter package tests to use the renamed APIs and keep the existing capability semantics unchanged.
- Update factory tests and any evidence-construction tests that inspect agent metadata.
- Update e2e assertions in `cmd/tessariq/run_e2e_test.go` to assert on `supported`.
- Update any integration tests that construct `agent.json` fixtures so they use the renamed field and updated contract expectations.
- Run the repository-wide unit and e2e suites listed in front matter.
- Run mutation testing because this task touches small but user-visible evidence-serialization logic.
- Run manual testing against a real emitted `agent.json` artifact and confirm the old key is absent.

## TDD Plan

1. RED: update `AgentInfo` JSON-shape tests to expect `supported` and reject `applied`.
2. RED: update adapter, factory, and e2e assertions to use the renamed field and method names.
3. GREEN: rename the implementation types, methods, comments, and JSON tags while preserving current capability semantics.
4. GREEN: update the v0.1.0 spec, README, and changelog to use `supported` consistently and to document the intentional breaking rename.
5. REFACTOR: remove stale `applied` wording and helper names across the codebase so only `supported` remains.
6. VERIFY: run the required automated suites, mutation testing, manual testing, and workflow verification.

## Notes

- `supported` is preferred over `capabilities` because the field is a narrow per-option support map, not a general inventory of everything an agent can do.
- This task intentionally removes `applied` outright rather than emitting both names.
- The repository is explicitly allowing this breaking evidence-contract change without a schema-version bump.
- Likely touched files include `specs/tessariq-v0.1.0.md`, `internal/adapter/agent.go`, `internal/adapter/agent_iface.go`, `internal/adapter/claudecode/claudecode.go`, `internal/adapter/opencode/opencode.go`, `internal/adapter/*_test.go`, `cmd/tessariq/run_e2e_test.go`, `README.md`, `CHANGELOG.md`, and `planning/tasks/TASK-017-v0-1-0-spec-conformity-closeout.md`.
- 2026-04-07T17:57:10Z: Renamed agent.json applied→supported across spec, implementation, tests, and docs. All unit tests pass, mutation efficacy 85.14%, manual testing 6/6 pass, zero verification findings.
