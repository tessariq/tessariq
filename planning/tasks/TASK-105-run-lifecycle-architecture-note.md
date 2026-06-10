---
id: TASK-105-run-lifecycle-architecture-note
title: Add an architecture note for the run lifecycle and evidence ownership
status: blocked
priority: p3
depends_on:
    - TASK-101-extract-run-orchestrator
    - TASK-102-evidence-package-and-atomic-json
milestone: v0.2.0
spec_version: v0.2.0
spec_refs:
    - specs/tessariq-v0.2.0.md#shared-runtime-sketch
    - specs/tessariq-v0.2.0.md#runner-responsibilities
    - specs/tessariq-v0.2.0.md#evidence-additions
updated_at: "2026-06-10T00:00:00Z"
areas:
    - docs
verification:
    unit:
        required: false
        commands:
            - go test ./...
        rationale: Documentation-only change; no code under test.
    integration:
        required: false
        commands:
            - go test -tags=integration ./...
        rationale: No code paths change.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: No code paths change.
    mutation:
        required: false
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: No production code changes.
    manual_test:
        required: false
        commands: []
        rationale: Documentation-only; reviewed by reading, not executed.
---

## Summary

Add a short architecture document (e.g. `docs/architecture/run-lifecycle.md`) describing the run lifecycle and the evidence ownership model, so the boundaries introduced by `TASK-101` (run orchestrator) and `TASK-102` (`internal/evidence`) are documented before v0.2.0 resume and runtime behavior expand.

## Motivation

Runtime/container behavior is spread across command assembly, adapters, container management, workspace provisioning, proxy setup, and runner lifecycle. Once the run orchestration is extracted and evidence schema ownership is centralized, a concise written map of "what owns what, in what order" prevents the new seams from eroding as contributors add workspace modes, resume rules, and new evidence artifacts. This is review priority #5.

## Acceptance Criteria

- A new doc under `docs/` (suggested `docs/architecture/run-lifecycle.md`) covers:
  - The end-to-end run flow from `tessariq run` invocation through orchestrator, provisioning, container/proxy setup, runner phases, and terminal evidence, naming the owning package at each step.
  - The evidence ownership model: which package writes each artifact, where atomic writes and completeness validation live (`internal/evidence`), and the read-only/host-mount contract boundaries.
  - The orchestrator boundary from `TASK-101`: what belongs in `cmd/` (flags, wiring, output) versus `internal/run` (lifecycle).
  - A short pointer to the v0.2.0 workspace modes and where they plug into the lifecycle.
- The doc is linked from `AGENTS.md` (source-of-truth files section) and/or `docs/` index so it is discoverable.
- No code changes; specs under `specs/` are not modified.

## Non-Goals

- Not a spec. It is informative architecture documentation; `specs/` remains normative.
- No exhaustive per-function reference; keep it a readable map, not generated API docs.

## Test Expectations

- Documentation-only; no automated tests.
- Verified by cross-checking each named package owner against the actual code after `TASK-101` and `TASK-102` land.
- Markdown links resolve and the `AGENTS.md`/`docs/` index reference is present.

## TDD Plan

- Documentation task; no test loop. Validate by cross-checking each named owner against the actual package after `TASK-101` and `TASK-102` land, so the doc matches code rather than intent.

## Notes

- Blocked until `v0.2.0` becomes the active milestone; depends on `TASK-101` and `TASK-102` so it documents the post-extraction structure rather than the current hotspot.
- Keep it short and high-signal; a stale long doc is worse than a concise accurate one.
