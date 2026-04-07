---
id: TASK-082-clarify-applied-field-semantics
title: Align applied-field semantics between spec, code comments, and implementation
status: done
priority: p2
depends_on:
    - TASK-008-adapter-contract-and-adapter-json
    - TASK-080-opencode-interactive-support
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-04-07T15:00:01Z"
areas:
    - adapters
    - evidence
    - spec
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Any semantic change to applied must be reflected in adapter unit tests.
    integration:
        required: false
        commands: []
        rationale: No new subsystem boundaries; applied semantics are tested at unit and e2e level.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: E2e tests assert applied values in agent.json evidence artifacts.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Applied-flag logic is trivially mutatable.
    manual_test:
        required: false
        commands: []
        rationale: This is a documentation and semantic alignment task; automated tests cover correctness.
---

## Summary

The `applied` field in `agent.json` has divergent definitions across spec, code comments, and implementation.

**Spec** (`specs/tessariq-v0.1.0.md:275`):
> "if an option such as `--model` or `--interactive` cannot be applied exactly, the selected agent MUST record that it was requested but not applied"

This implies per-run semantics: "was this specific option forwarded to the agent in this run?"

**Code comment** (`internal/adapter/agent.go:19-20`):
> "applied records whether each requested option was successfully applied by the agent"

Also implies per-run.

**Implementation** (`internal/adapter/claudecode/claudecode.go`, `internal/adapter/opencode/opencode.go`):
Both adapters return static values in `buildApplied()` independent of `cfg.Interactive`. Claude Code always returns `"interactive": true`; OpenCode (post-TASK-080) also returns `true`. Neither checks whether interactive was actually requested. This is capability-flag behavior.

**Spec example** (`specs/tessariq-v0.1.0.md:435-438`):
Shows `applied.model = false` for an unsupported model, `applied.interactive = true` for a supported option — consistent with capability semantics, but ambiguous when the option is not requested.

## Acceptance Criteria

- One interpretation is chosen (capability flag or per-run applied).
- Spec language at line 275 is updated to match the chosen interpretation.
- Spec example at lines 425-439 is updated if needed (e.g., to show a case where an option is not requested but applied is still present).
- Code comment in `internal/adapter/agent.go:19-20` matches the chosen interpretation.
- `buildApplied` doc comments in both adapters match the chosen interpretation.
- All existing tests remain green with no assertion changes (or assertions are updated to match the chosen semantics).
- If per-run semantics are chosen: `buildApplied` must check `cfg.Interactive` and only return `true` when interactive is both requested and supported.

## Test Expectations

- Add/adjust unit tests in adapter packages so `applied` expectations match the selected semantics.
- Keep `cmd/tessariq` tests and e2e artifact assertions consistent with the selected semantics for `requested` vs `applied`.
- Verify no regressions by running the commands listed in `verification.unit` and `verification.e2e`.

## TDD Plan

1. RED: add or adjust adapter tests to encode the chosen `applied` semantics.
2. GREEN: update implementation and comments/spec text until the new tests pass.
3. REFACTOR: align remaining tests (including e2e assertions) and remove ambiguity in docs/comments.
4. VERIFY: run the required verification commands from this task's front matter.

## Notes

- This divergence has existed since TASK-008 and was surfaced during TASK-080 code review.
- The capability-flag interpretation has been the de-facto behavior since day one — changing to per-run would require implementation changes in both adapters.
- The capability-flag interpretation makes `applied` useful for tooling that needs to know "what can this adapter do?" without needing to cross-reference `requested`.
- Key files: `specs/tessariq-v0.1.0.md`, `internal/adapter/agent.go`, `internal/adapter/claudecode/claudecode.go`, `internal/adapter/opencode/opencode.go`.
- 2026-04-07T15:00:01Z: Aligned agent.json applied semantics as capability flags; go test ./..., go test -tags=e2e ./..., gremlins efficacy 85.11%, manual test planning/artifacts/manual-test/TASK-082-clarify-applied-field-semantics/20260407T145717Z/report.md, verify planning/artifacts/verify/task/TASK-082-clarify-applied-field-semantics/20260407T145952Z/report.json
