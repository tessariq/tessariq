---
id: TASK-076-pin-default-agent-images-by-digest
title: Pin default agent images by digest
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#tessariq-run-task-path
dependencies:
    - TASK-024-claude-code-agent-runtime-integration
    - TASK-025-opencode-agent-runtime-integration
    - TASK-063-pin-squid-proxy-image-by-digest
updated_at: "2026-04-05T08:45:18Z"
---

## Summary

The default Claude Code and OpenCode runtime images still use mutable `:latest` tags even though the Squid proxy image and workspace-repair image are already digest-pinned. Pin the remaining default agent images by digest so the normal `tessariq run` path no longer depends on mutable tags.

## Supersedes

- BUG-042 from `planning/BUGS.md`.

## Acceptance Criteria

- `claudecode.DefaultImage` uses a digest-pinned image reference.
- `opencode.DefaultImage` uses a digest-pinned image reference.
- Existing `--image` override behavior remains unchanged.
- Automated tests fail if either default agent image regresses back to a mutable tag.

## Test Expectations

- Add unit coverage or constant-guard tests asserting both default image references contain `@sha256:`.
- Keep existing adapter/runtime tests passing without broadening process-level coverage unless a new validation helper is introduced.

## TDD Plan

1. RED: add a failing test that asserts both default agent images are digest-pinned.
2. GREEN: replace the mutable default image tags with pinned digests.
3. GREEN: rerun runtime and adapter tests to confirm default image resolution still works.

## Notes

- Likely files: `internal/adapter/claudecode/claudecode.go`, `internal/adapter/opencode/opencode.go`, and nearby adapter/runtime tests.
- Match the existing Squid-image pattern instead of introducing a new image-reference abstraction unless reuse becomes obvious.
- 2026-04-05T08:45:18Z: DefaultImage constants pinned by digest for both claude-code and opencode adapters; guard tests added; agent Dockerfiles and CI workflow for image building/testing/pushing created
