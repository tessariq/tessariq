---
id: TASK-101-extract-run-orchestrator
title: Extract run lifecycle orchestration out of the Cobra run command
status: blocked
priority: medium
spec_ref: specs/tessariq-v0.2.0.md#shared-runtime-sketch
dependencies:
    - TASK-017-v0-1-0-spec-conformity-closeout
updated_at: "2026-06-10T00:00:00Z"
---

## Summary

Extract the run lifecycle currently embedded in the `newRunCmd` `RunE` closure (`cmd/tessariq/run.go`, ~340 lines) into a typed orchestrator in `internal/run` (for example `run.Execute(ctx, cfg) (run.Result, error)`). Cobra keeps only flag parsing, command wiring, and output formatting. This is a behavior-preserving refactor; no user-visible behavior changes.

## Motivation

`cmd/tessariq/run.go` is the repository's primary integration hotspot. Its `RunE` path owns prerequisites, task validation, clean-repo gating, egress resolution, allowlist resolution, auth discovery, runtime probing, worktree provisioning, runtime-state setup, agent update, process construction, runner execution, index updates, cleanup, and user output. Because the logic lives inside a Cobra closure, the run pipeline cannot be unit-tested without driving the command, forcing all coverage into slower integration and e2e tests.

v0.2.0 adds three workspace modes (`worktree`, `copy+patch`, `repo-rw`) plus resume rules, all of which branch on the same lifecycle. Without extraction first, this file becomes the highest-risk change point in the codebase. This task is the structural prerequisite for the v0.2.0 workspace-mode work.

## Acceptance Criteria

- A new exported orchestrator in `internal/run` (e.g. `run.Execute`) accepts a `context.Context` and a `run.Config`-derived input and returns a typed result plus error. It does not import `cmd/` and does not reference Cobra.
- `cmd/tessariq/run.go`'s `RunE` is reduced to: build config from flags, resolve repo root, call the orchestrator, and format output. Target: the `RunE` closure is well under the 50-line function guideline.
- Business logic currently living in `cmd/` (`resolveAllowlistCore`, `resolveRunAllowlist`, `appendRunningIndexEntry`, `appendIndexEntry`, `requiredImageBinaries`, allowlist/index helpers) moves into `internal/run` (or an appropriate internal package) and is exercised by unit tests.
- External collaborators (prereq checker, git operations, workspace provisioning, runner) are reached through injectable seams so the orchestrator is unit-testable without Docker, git, or the filesystem, consistent with the existing `runner.ProcessRunner` / `runner.SessionStarter` injection style.
- User-facing output, exit codes, and the set/shape of emitted evidence artifacts are unchanged. The clean-repo gate, egress resolution precedence, and `manifest.json` provenance fields behave identically.
- All existing run-path unit, integration, and e2e tests pass without modification to their assertions on behavior (test wiring may be updated to call the new entrypoint).

## Non-Goals

- No new CLI flags, no new workspace modes, no resume behavior. Those are separate v0.2.0 tasks that build on this seam.
- No change to evidence schemas or artifact contents.
- No broad rename of existing internal packages beyond moving the listed helpers.

## Test Expectations

- Unit tests drive `run.Execute` with injected prereq/git/provision/runner fakes, asserting orchestration ordering and the typed result without Cobra, Docker, or the filesystem.
- Unit tests for the relocated allowlist and index helpers covering egress precedence and index-append behavior.
- Integration and e2e suites prove the `run -> attach -> promote` flow, output, exit codes, and evidence artifacts are unchanged.
- Mutation testing guards the moved lifecycle and cleanup branches.

## TDD Plan

- Start RED: add a unit test in `internal/run` that drives `run.Execute` with injected fakes for prereq/git/provision/runner and asserts the orchestration ordering and a successful typed result. It fails because `run.Execute` does not exist.
- GREEN: introduce `run.Execute` by moving the `RunE` body into it behind the injected seams; thin `RunE` down to call it.
- REFACTOR: relocate the `cmd/`-resident helpers into `internal/run`, fold their existing tests, and confirm the command layer no longer holds business logic.
- Keep each step behavior-preserving; run the integration and e2e suites after the move to prove no regression.

## Notes

- Blocked until `v0.2.0` becomes the active milestone (mirrors the milestone-gating convention used by `TASK-019`). Depends on the v0.1.0 closeout (`TASK-017`) landing first so the refactor starts from a verified-conformant baseline.
- This is recommended priority #1 in `REVIEW.md` ("Extract user-facing run orchestration from `cmd/tessariq/run.go` before adding v0.2 workspace modes").
- Pairs naturally with a short architecture note for the run lifecycle (see proposed follow-up task) so the new orchestrator boundary is documented before resume/runtime behavior expands.
