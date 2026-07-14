---
id: TASK-007-attach-command-live-run-resolution
title: Implement attach command and live run resolution
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#host-prerequisites
dependencies:
    - TASK-006-tmux-session-and-detached-attach-guidance
    - TASK-014-run-index-and-run-ref-resolution
updated_at: "2026-04-01T17:16:44Z"
---

## Summary

Implement `tessariq attach <run-ref>` on top of the shared repository-scoped run-ref resolver.

## Acceptance Criteria

- Attach accepts explicit `run_id`, `last`, and `last-N` via the shared repository-scoped run-ref resolver.
- Attach works only for live runs.
- Unknown or finished runs fail cleanly without attaching, tell the user the run is not live, and include the evidence path when it is known.
- Attach fails cleanly with actionable guidance when `tmux` is missing or unavailable on the host.

## Test Expectations

- Add unit tests for attach preflight decisions and live-run eligibility on top of the shared run-ref resolver.
- Add integration tests for live-session lookup and attach failures using Testcontainers-backed collaborators only.
- Add unit tests for missing-`tmux` prerequisite handling and user guidance.
- Add a thin e2e attach flow for a live run.
- Add an error-path e2e test that verifies actionable missing-`tmux` guidance.
- Run mutation testing because the resolution logic has multiple branches.

## TDD Plan

- Start with a failing unit test for attach live-run eligibility, then a failing e2e test for attaching to a live run.

## Notes

- Shared run-ref parsing and index semantics are intentionally owned by `TASK-014`.
- This task is not materially changed by the v0.1.0 agent/runtime spec shift; it remains in the v0.1.0 backlog unchanged except for refreshed planning metadata.
- 2026-04-01T17:10:00Z: Change note: include a minimal user-facing tmux detach hint (`Ctrl-b d`) in the attach UX/help text so attached users can leave the live session without stopping the run.
- 2026-04-01T17:16:44Z: Implemented attach live-run resolution; evidence: go vet ./..., go test ./..., go test -tags=integration ./..., go test -tags=e2e ./..., gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70, (evidence artifacts; path omitted) (evidence artifacts; path omitted)
