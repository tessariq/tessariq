---
id: TASK-076-pin-default-agent-images-by-digest
title: Pin default agent images by digest
status: done
priority: p1
depends_on:
    - TASK-024-claude-code-agent-runtime-integration
    - TASK-025-opencode-agent-runtime-integration
    - TASK-063-pin-squid-proxy-image-by-digest
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
updated_at: "2026-04-05T08:45:18Z"
areas:
    - runtime
    - security
    - supply-chain
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Default image selection and digest guardrails should start with focused tests.
    integration:
        required: false
        commands: []
        rationale: The change is constant and validation logic, not a new process interaction.
    e2e:
        required: false
        commands: []
        rationale: Existing runtime integration coverage should remain sufficient once the defaults are pinned.
    mutation:
        required: false
        commands: []
        rationale: The task is mostly constant replacement plus guard tests.
    manual_test:
        required: false
        commands: []
        rationale: No manual-only behavior is expected once automated guards prove the defaults are pinned.
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
