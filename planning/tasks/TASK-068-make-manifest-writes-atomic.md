---
id: TASK-068-make-manifest-writes-atomic
title: Write manifest.json atomically to avoid crash-corrupted evidence
status: completed
priority: low
spec_ref: specs/tessariq-v0.1.0.md#evidence-contract
dependencies:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-032-container-security-hardening
updated_at: "2026-04-03T08:50:48Z"
---

## Summary

`WriteManifest` still uses direct `os.WriteFile`, unlike `WriteStatus` which already uses a safer temp-file-plus-rename pattern. Align manifest writes with the existing atomic evidence-writing contract so a crash cannot leave partially written JSON behind.

## Supersedes

- BUG-035 from `planning/BUGS.md`.

## Acceptance Criteria

- `manifest.json` is written via a temp file and atomic rename in the evidence directory.
- Failed writes do not leave a partial `manifest.json` behind.
- File permissions remain `0o600` and directory permissions remain unchanged.
- Existing manifest read behavior is unchanged for successful writes.

## Test Expectations

- Add unit tests covering successful atomic writes and cleanup on failure.
- Add regression coverage that permissions and JSON formatting remain unchanged.

## TDD Plan

1. RED: add a write-path test that requires temp-file-plus-rename semantics.
2. GREEN: switch `WriteManifest` to the same atomic pattern used by status writing.
3. GREEN: keep existing manifest shape and permissions unchanged.

## Notes

- Likely files: `internal/run/manifest.go` and manifest tests.
- Prefer reusing the same helper pattern as `runner.WriteStatus` if doing so stays local and readable.
- 2026-04-03T08:50:48Z: WriteManifest now uses temp-file+rename atomic pattern matching WriteStatus; unit tests added for no-temp-file-leak and overwrite semantics; mutation efficacy 85.85%; all 4 manual tests pass
