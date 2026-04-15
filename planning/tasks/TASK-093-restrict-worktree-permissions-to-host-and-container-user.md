---
id: TASK-093-restrict-worktree-permissions-to-host-and-container-user
title: Restrict worktree permissions to the host and container user only
status: done
priority: p1
depends_on:
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-027-container-lifecycle-and-mount-isolation
    - TASK-061-cleanup-worktrees-even-when-ownership-repair-fails
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-15T19:51:26Z"
areas:
    - workspace
    - container
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Permission bits and path contracts should be pinned deterministically where possible.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The fix spans a host-side chmod (or chown) plus Docker bind-mount UID semantics, which require real filesystem coverage.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Worktree access is observable to any concurrent shell on the host and should be verified from a full CLI run.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: A weakened permission check can still make every happy-path test pass.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check on a multi-user host should confirm that a second local user cannot read or modify a live worktree.
---

## Summary

Stop opening live worktrees to every local user on the host. `container.Process.prepareWritableMounts` today runs `chmod -R a+rwX` on every writable bind-mount source, and `workspace.Provision` creates the parent directory chain with `0o755`. On a shared host any local user can enumerate live run IDs, read the files the agent is mutating, and tamper with them mid-run — those edits then flow into `diff.patch` and into the promoted commit. This contradicts the rest of the hardening posture (read-only auth mounts, cap-drop, non-root container user).

## Supersedes

- BUG-058 from `planning/BUGS.md`.

## Acceptance Criteria

- Live worktrees under `~/.tessariq/worktrees/<repo_id>/<run_id>/` are accessible only to the host user that owns the tessariq process and to the container's non-root `tessariq` user. Other local users on the host cannot read or write files inside the worktree.
- The parent directory chain `~/.tessariq/worktrees/` and `~/.tessariq/worktrees/<repo_id>/` is created with at most `0o700` (or an equivalent ACL) so a non-owner cannot even enumerate run IDs.
- The container's `tessariq` user still has full read/write access to the bind-mounted worktree at `/work`, so existing agent behavior and evidence generation are preserved.
- Cleanup on both success and failure paths still works even when the host user and container user differ in UID (BUG-028/TASK-061 guarantees remain intact).
- The fix picks one reproducible mechanism — chown-to-container-uid, POSIX ACLs, or a dedicated group — and documents the choice in code comments and the task completion note. No mechanism may require CAP_SYS_ADMIN on the host outside of Docker itself.
- `runtime.json` or related evidence continues to reflect accurate host-side mount policy; no evidence contract drift.

## Test Expectations

- Start with a failing unit/integration test that provisions a worktree and asserts the resulting permissions are no wider than host-user plus container-user access.
- Add a test that attempts to read a worktree file as a second user (or an equivalent permission assertion without requiring a real second user) and proves the read is denied.
- Add integration coverage that runs a full agent container against the hardened worktree and verifies the agent still writes successfully via the container's `tessariq` user.
- Add cleanup coverage that exercises both success and failure paths with the new permission scheme so TASK-061's ownership-repair fallback still fires when needed.
- Run mutation testing on the permission-assembly helpers.
- Manual test: on a real multi-user host, start a run as user A, then from user B attempt `ls`, `cat`, and `echo >>` against the worktree and verify all three fail.

## TDD Plan

1. RED: pin the current over-permissive chmod with a failing test expecting no-other-user access.
2. GREEN: pick the mechanism (chown to a reserved UID/GID, ACLs, or dedicated group) and implement it in `workspace.Provision` and/or `container.Process.prepareWritableMounts`.
3. GREEN: tighten worktree parent-dir creation modes in `workspace.Provision`.
4. GREEN: make sure cleanup still succeeds across the UID mismatch by reusing or extending the existing ownership-repair helper.
5. VERIFY: rerun the full automated ladder plus the multi-user manual check.

## Notes

- Do not weaken the container-user non-root discipline. The goal is "host user + container user", not "host user only".
- Do not re-grant world access through any intermediate `chmod` step — the entire lifecycle must keep the worktree contained.
- Coordinate with the runtime-state layer (TASK-087): both paths now create per-run, non-world-readable scratch trees, and should follow consistent permission discipline.
- Prefer a mechanism that works without host-level root privileges, since tessariq runs as an unprivileged user on developer machines.
- 2026-04-15T18:53:14Z: Implemented TASK-093 worktree hardening and runtime-state scratch hardening. Manual test pass: planning/artifacts/manual-test/TASK-093-restrict-worktree-permissions-to-host-and-container-user/20260415T185042Z/report.md. Automated checks rerun: go test ./..., go test -tags=integration ./..., go test -tags=e2e ./..., go vet ./..., go run ./cmd/tessariq --help, go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json, gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70.
- 2026-04-15T19:51:26Z: Reworked TASK-093 after PR review: runtime image compatibility now probes the numeric identity of the named tessariq user, worktree and runtime-state hardening use owner-only mode bits plus exact-principal POSIX ACLs on Linux, Provision and runtime-state setup clean up on hardening failure, and custom images without uid 1000 now work while images missing tessariq fail fast. Manual test pass: planning/artifacts/manual-test/TASK-093-restrict-worktree-permissions-to-host-and-container-user/20260415T194735Z/report.md. Automated checks rerun: gofmt -l ., go vet ./..., go test ./..., go test -tags=integration ./..., go test -tags=e2e ./..., go run ./cmd/tessariq --help, go run ./cmd/tessariq-workflow check-skills, go run ./cmd/tessariq-workflow verify --profile task --task TASK-093-restrict-worktree-permissions-to-host-and-container-user --disposition report --json, go run ./cmd/tessariq-workflow verify --profile spec --disposition report --json, gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70.
