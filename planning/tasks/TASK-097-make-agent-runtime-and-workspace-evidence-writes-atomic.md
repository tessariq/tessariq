---
id: TASK-097-make-agent-runtime-and-workspace-evidence-writes-atomic
title: Write agent.json runtime.json and workspace.json atomically to avoid crash-corrupted evidence
status: done
priority: p2
depends_on:
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-068-make-manifest-writes-atomic
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-24T08:01:24Z"
areas:
    - evidence
    - reliability
    - filesystem
    - workspace
    - adapters
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Atomic file-writing behavior is deterministic and belongs under focused unit coverage.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The evidence contract depends on real filesystem write, overwrite, and rename behavior, so integration coverage is justified for the I/O path.
    e2e:
        required: false
        commands: []
        rationale: Focused writer coverage is sufficient for this local evidence-write hardening.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Small write-path branches are easy to weaken while preserving happy-path behavior.
    manual_test:
        required: false
        commands: []
        rationale: This crash-safety pattern is better validated by automated filesystem tests than by ad-hoc manual interruption.
---

## Summary

`workspace.json`, `agent.json`, and `runtime.json` still use direct `os.WriteFile`, unlike `manifest.json` and `status.json`, which already use temp-file-plus-rename atomic writes. Align these required JSON evidence artifacts with the same atomic-write discipline so a crash or interrupted write cannot leave partial or empty evidence files behind.

## Supersedes

- BUG-053 from `planning/BUGS.md`.

## Acceptance Criteria

- `workspace.json`, `agent.json`, and `runtime.json` are written via a temp file and atomic rename in the evidence directory.
- Failed writes do not leave a partial target file behind.
- Temporary files are cleaned up on error.
- File permissions remain `0o600`, directory permissions remain unchanged, and successful JSON contents are unchanged.
- The implementation uses one shared local pattern or helper where that improves consistency without adding unnecessary abstraction.

## Test Expectations

- Add unit tests for successful atomic writes for all three artifacts.
- Add failure-path tests proving no partial target file remains after a write failure or rename failure.
- Add integration tests that exercise real filesystem writes and prove temp-file cleanup, overwrite semantics, rename behavior, and final `0o600` permissions.
- Add regression coverage that JSON formatting and field contents remain unchanged.
- Run mutation testing because write-failure cleanup logic is easy to under-test.

## TDD Plan

1. RED: add failing tests that require temp-file-plus-rename semantics for `workspace.json`, `agent.json`, and `runtime.json`.
2. GREEN: switch each writer to the same atomic pattern already used by `manifest.json` and `status.json`.
3. GREEN: keep artifact schema, formatting, and permissions unchanged.
4. VERIFY: rerun writer-focused unit and integration coverage plus the broader unit suite.

## Notes

- Likely files: `internal/workspace/metadata.go`, `internal/adapter/agent.go`, `internal/adapter/runtime.go`, and their tests.
- Prefer the existing repo-local atomic-write pattern over inventing a generalized utility unless the duplication becomes clearly worse.
- Add this task to `TASK-017-v0-1-0-spec-conformity-closeout` dependencies so the release gate cannot run before BUG-053 is resolved.
- 2026-04-24T08:01:24Z: workspace.json, agent.json, runtime.json now use temp-file+rename atomic writes matching manifest.json/status.json pattern; 6 regression tests added; mutation testing 86.5% efficacy; manual tests 5/5 pass
