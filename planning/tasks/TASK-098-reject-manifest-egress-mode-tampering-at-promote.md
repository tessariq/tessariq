---
id: TASK-098-reject-manifest-egress-mode-tampering-at-promote
title: Reject manifest egress-mode tampering that suppresses proxy evidence requirements
status: done
priority: p1
depends_on:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-048-promote-manifest-run-identity-consistency
    - TASK-088-require-proxy-evidence-completeness-before-promote
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-24T08:44:30Z"
areas:
    - promote
    - proxy
    - evidence
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: The trusted-source mode decision and mismatch checks are deterministic and should be pinned at unit level first.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Promote rejection must be proven against tampered evidence in a real git repository.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: This is a user-visible promote trust boundary and should hold end to end.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Conditional trust gates are easy to weaken accidentally while keeping happy paths green.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check should show that a proxy run relabeled to direct in `manifest.json` is rejected before any git side effects.
---

## Summary

`runner.CheckEvidenceCompleteness` currently decides whether proxy artifacts are required entirely from the mutable `manifest.json.resolved_egress_mode` field. A proxy-mode run can be relabeled to `direct`, which suppresses the proxy evidence requirement and lets `promote` accept a run without its required proxy telemetry. Harden the promote/evidence contract so proxy evidence requirements come from trusted run state, not a single editable manifest field.

## Supersedes

- BUG-062 from `planning/BUGS.md`.

## Acceptance Criteria

- Proxy-evidence requirements are driven from a trusted source of resolved run mode rather than trusting `manifest.json.resolved_egress_mode` alone.
- If the trusted run mode says proxy, `promote` rejects the run when `egress.compiled.yaml` or `egress.events.jsonl` is missing, even if `manifest.json` has been rewritten to `direct`.
- If the trusted run mode and `manifest.json` disagree, Tessariq fails closed with tamper/inconsistency messaging before any git side effects.
- Honest direct-mode and proxy-mode runs continue to behave as before.
- The implementation uses one clear trust boundary for promote-time mode checks so completeness logic does not drift.

## Test Expectations

- Add unit coverage for trusted-mode completeness decisions, including a proxy run with tampered manifest mode.
- Add promote integration coverage showing that a proxy-mode run relabeled to `direct` in `manifest.json` is rejected before branch creation.
- Add regression coverage that honest direct-mode runs still do not require proxy artifacts.
- Add regression coverage that honest proxy-mode runs with intact artifacts still promote successfully.
- Run mutation testing because conditional trust checks are branch-heavy and security-sensitive.

## TDD Plan

1. RED: add failing tests that show a proxy-mode run can currently bypass proxy-evidence requirements by rewriting `manifest.json.resolved_egress_mode`.
2. GREEN: introduce a trusted source for resolved egress mode and reject mode mismatch before promote side effects.
3. GREEN: route completeness decisions through that trusted source.
4. VERIFY: rerun unit, integration, e2e, mutation, and manual tamper checks.

## Notes

- Prefer a minimal trustable source over duplicating mode logic in multiple places.
- Keep the error style aligned with existing tampered-evidence failures from TASK-048 and TASK-089.
- Add this task to `TASK-017-v0-1-0-spec-conformity-closeout` dependencies so closeout cannot pass while proxy-evidence requirements are bypassable by manifest tampering.
- 2026-04-24T08:44:30Z: Hardened promote to cross-check resolved_egress_mode between manifest.json and runtime.json. Added ErrEgressModeMismatch sentinel, runtime.json now records resolved_egress_mode, CheckEvidenceCompleteness cross-validates both sources and fails closed on mismatch. Unit (18 tests), integration (30 tests), e2e (11 tests), mutation (86.65% efficacy), and manual (6/6 pass) all green.
