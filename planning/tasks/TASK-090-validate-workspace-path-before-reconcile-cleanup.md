---
id: TASK-090-validate-workspace-path-before-reconcile-cleanup
title: Validate workspace path before reconcile triggers cleanup
status: done
priority: p0
depends_on:
    - TASK-004-worktree-provisioning-and-workspace-metadata
    - TASK-051-attach-repo-local-evidence-path-validation
    - TASK-061-cleanup-worktrees-even-when-ownership-repair-fails
    - TASK-085-harden-run-finalization-and-orphaned-run-recovery
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#tessariq-attach-run-ref
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-15T12:34:19Z"
areas:
    - lifecycle
    - workspace
    - attach
    - promote
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Path-containment checks on an untrusted evidence field should be pinned deterministically at the unit level.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Reconcile drives real Docker chown/chmod and host-side filesystem teardown; integration coverage must exercise the full workspace cleanup path.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: attach and promote are user-visible commands and both invoke reconcile with workspace cleanup on non-success runs.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Containment predicates are easy to weaken to a string-prefix that still passes happy-path tests.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check must confirm that a tampered `workspace.json` cannot redirect `chown`/`chmod`/`os.RemoveAll` to a path outside the canonical per-run worktree tree.
---

## Summary

Stop `lifecycle.cleanupTerminalRun` from trusting the `workspace_path` field read out of `workspace.json`. A tampered or symlinked evidence artifact can currently redirect `workspace.Cleanup` at an attacker-chosen host path, which performs `docker run` as root with a bind mount (chowning the target to the host user), a host-side `chmod -R u+rwX`, and `os.RemoveAll`. Evidence-path reads have already been hardened by BUG-013/016/054; the symmetric write path inside reconcile was never audited for the same containment contract.

## Supersedes

- BUG-055 from `planning/BUGS.md`.

## Acceptance Criteria

- `lifecycle.cleanupTerminalRun` (and any helper it calls) must never pass an untrusted, evidence-sourced filesystem path to `workspace.Cleanup`. The canonical per-run workspace path is recomputed from trusted inputs (`homeDir`, `repoRoot`, `manifest.run_id`) via `workspace.WorkspacePath` before any cleanup step runs.
- Reconcile refuses to proceed with cleanup if the recomputed canonical path disagrees with the value stored in `workspace.json`, surfacing the mismatch through the existing reconcile/attach/promote error contract rather than silently "fixing" it.
- The recomputed path is resolved against the real filesystem (symlinks inside `~/.tessariq/worktrees/<repo_id>/<run_id>` must not allow escape), mirroring the approach used for task-path and evidence-path containment.
- `workspace.Cleanup` itself treats its `workspacePath` argument as untrusted defensive input: it refuses to chown, chmod, or `os.RemoveAll` any path that is not contained within `~/.tessariq/worktrees/` after symlink resolution.
- `attach`, `promote`, and any background reconcile entry point exercise the hardened path. A non-success reconcile for a run whose `workspace.json` has been tampered with fails cleanly without side effects on the host filesystem.
- Legitimate runs still clean up their worktree exactly as before, preserving the existing BUG-028/TASK-061 behavior (ownership repair failure does not block git worktree removal and directory deletion).

## Test Expectations

- Start with a failing unit test that plants a `workspace.json` pointing at a target outside the canonical `~/.tessariq/worktrees/` tree and asserts that reconcile refuses to touch it.
- Add unit coverage for the new `workspace.Cleanup` defensive guard: inputs outside the canonical tree must return a containment error without executing any Docker, chmod, or removal step.
- Add integration coverage that exercises `lifecycle.ReconcileRun` end-to-end and proves a non-success run with a tampered `workspace.json` does not invoke the docker/chmod/remove pipeline.
- Add attach- or promote-level tests that simulate a tampered evidence directory and show the command fails before reaching cleanup.
- Run mutation testing because containment predicates in security-sensitive cleanup code are easy to weaken accidentally.
- Manual test against a built CLI: plant a `workspace.json` with `workspace_path` set to a non-canonical path under `/tmp`, trigger `tessariq attach <run-id>`, and verify the path is untouched afterwards.

## TDD Plan

1. RED: reproduce the arbitrary-path primitive with a unit test that points a tampered `workspace.json` at a decoy directory and asserts reconcile rejects it.
2. GREEN: teach reconcile to recompute the canonical workspace path from trusted inputs and to refuse mismatches before calling cleanup.
3. GREEN: add a defensive containment check inside `workspace.Cleanup` so the guarantee holds even if a future caller forgets to pre-validate.
4. REFACTOR: extract a small `ValidateWorkspacePath` helper (mirroring `ValidateEvidencePath`) to keep the contract in one place.
5. VERIFY: rerun the full automated ladder plus the manual tampered-evidence check.

## Notes

- Do not "fix" the mismatch by silently overwriting `workspace.json`; the spec-facing contract is that tampered evidence is rejected, not normalized.
- Keep the defensive check inside `workspace.Cleanup` minimal — it is a safety net, not a replacement for validating in the caller.
- The canonical path format is already fixed by `workspace.WorkspacePath(homeDir, repoRoot, runID)`; reuse it instead of inventing a second shape.
- Coordinate with TASK-089's symlink-resolution approach so the new workspace-path check uses the same `filepath.EvalSymlinks` discipline.
- 2026-04-15T12:34:19Z: hardened reconcile workspace cleanup with canonical path validation; defensive containment in workspace.Cleanup; 100% workspace + 86% lifecycle mutation efficacy; manual test verified tampered workspace.json rejected without decoy side effects
