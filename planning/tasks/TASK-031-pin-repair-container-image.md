---
id: TASK-031-pin-repair-container-image
title: Pin workspace repair container image by digest
status: done
priority: p1
depends_on:
    - TASK-028-container-session-streaming-and-cleanup-hardening
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#workspace-guarantees
updated_at: "2026-04-01T10:56:08Z"
areas:
    - workspace
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Image reference construction should be unit-testable.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Repair container behavior crosses process boundaries and requires integration testing.
    e2e:
        required: false
        commands: []
        rationale: Existing e2e cleanup coverage is sufficient once the image is pinned.
    mutation:
        required: false
        commands: []
        rationale: Image reference is a constant, not branch logic.
    manual_test:
        required: false
        commands: []
        rationale: Image pinning is fully testable through automated tests.
---

## Summary

The workspace ownership repair function (`repairWorkspaceOwnership` in `internal/workspace/provision.go`) currently uses `alpine:latest` as the repair container image. This is a supply chain risk: a compromised `:latest` tag would run as root with the worktree bind-mounted. This task pins the image by digest per the v0.1.0 spec requirement.

## Supersedes

This task addresses a gap in TASK-028's implementation. TASK-028 correctly scoped repair to disposable worktree paths but did not specify or pin the container image.

## Acceptance Criteria

- The repair container image is pinned by digest (e.g., `alpine@sha256:<digest>`) rather than a mutable tag.
- The pinned digest is documented in a constant or config that is easy to update during maintenance.
- The repair container only mounts the disposable worktree path (no evidence, auth, or config mounts).
- Repair continues to run as root inside the container (required for `chown`).
- A failed image pull produces an actionable error message.

## Test Expectations

- Add unit tests verifying the image reference includes a digest, not a mutable tag.
- Add unit tests verifying only the worktree path is mounted in the repair container.
- Add integration tests for repair behavior using the pinned image.

## TDD Plan

1. RED: write unit test asserting the repair image reference contains `@sha256:` (digest pinning).
2. RED: write unit test asserting only the worktree path is mounted in the repair container create args.
3. GREEN: replace `alpine:latest` with a digest-pinned constant.
4. IMPROVE: ensure the pinned digest is in a well-named constant for easy maintenance.
5. RED: write integration test for repair behavior using the pinned image.
6. GREEN: verify integration test passes with the pinned image.

## Notes

- Files likely affected: `internal/workspace/provision.go` (`repairWorkspaceOwnership`), `internal/workspace/provision_test.go`, `internal/workspace/provision_integration_test.go`.
- Use `docker pull alpine:latest && docker inspect --format='{{index .RepoDigests 0}}' alpine:latest` to obtain the current digest for pinning.
- 2026-04-01T10:56:08Z: Pinned repair image by digest (alpine@sha256:25109184c71b...), extracted buildRepairArgs for testability, 6 unit tests + 3 integration tests pass, all 6 manual test steps pass
