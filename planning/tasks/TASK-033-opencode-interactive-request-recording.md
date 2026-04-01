---
id: TASK-033-opencode-interactive-request-recording
title: Allow OpenCode interactive requests and record not-applied semantics in agent evidence
status: done
priority: p1
depends_on:
    - TASK-025-opencode-agent-runtime-integration
    - TASK-029-interactive-runtime-mode-independent-of-attach
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-04-01T11:04:41Z"
areas:
    - cli
    - opencode
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Request/applied option semantics are easiest to pin with unit tests around config and agent info generation.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Run-level evidence generation must be validated end-to-end with containerized collaborators.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: User-visible CLI behavior changed for a previously rejected flag combination.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Guard logic around requested/applied fields is branchy and regression-prone.
    manual_test:
        required: true
        commands: []
        rationale: Verify final UX and evidence files with a real local run invocation.
---

## Summary

`--agent opencode --interactive` is currently rejected at CLI validation time, so runs cannot emit `agent.json` that records `interactive` as requested but not applied. This task removes the premature hard-reject and preserves spec-required requested/applied evidence semantics.

## Supersedes

- BUG-001 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq run --agent opencode --interactive ...` no longer fails during early config gating solely because `interactive` is unsupported by OpenCode.
- `agent.json` for OpenCode records `requested.interactive=true` and `applied.interactive=false` when interactive is requested.
- Existing OpenCode behavior for unsupported `--model` continues to record requested/applied semantics correctly.
- Existing Claude Code interactive behavior remains unchanged.
- CLI help and warnings remain accurate for interactive mode guidance.

## Test Expectations

- Add/adjust unit tests in run command validation to ensure OpenCode interactive is not hard-rejected.
- Add/adjust unit tests for OpenCode requested/applied evidence maps.
- Add integration test that executes an OpenCode run with `--interactive` and asserts evidence fields.
- Add e2e assertion that the command path proceeds and produces evidence instead of failing at argument validation.

## TDD Plan

1. RED: add failing unit test proving OpenCode interactive is currently rejected in run command path.
2. RED: add failing test asserting OpenCode `agent.json` requested/applied interactive fields when `--interactive` is set.
3. GREEN: remove/adjust run-level hard gate while preserving any necessary runtime guidance.
4. GREEN: keep OpenCode adapter requested/applied fields consistent with spec.
5. REFACTOR: simplify validation ownership between config validation and adapter/evidence generation.
6. GREEN: run integration/e2e checks for regression safety.

## Notes

- Likely files: `cmd/tessariq/run.go`, `cmd/tessariq/run_test.go`, `internal/adapter/opencode/opencode_test.go`, and run integration/e2e tests.
- Keep this task behavior-preserving outside the interactive gating change.
- 2026-04-01T11:04:41Z: Removed premature CLI hard-gate for opencode+interactive. OpenCode adapter already recorded requested/applied correctly; gate prevented it from running. Added non-fatal stderr warning, updated attach note to check Applied map, added factory unit test, transformed e2e test from failure to evidence assertion. All tests pass: unit, integration, e2e, mutation (84.95%). Manual tests 5/5 pass.
