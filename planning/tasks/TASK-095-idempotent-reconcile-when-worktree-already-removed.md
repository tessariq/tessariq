---
id: TASK-095-idempotent-reconcile-when-worktree-already-removed
title: Keep reconcile cleanup idempotent when the canonical worktree has already been removed
status: done
priority: p1
depends_on:
    - TASK-090-validate-workspace-path-before-reconcile-cleanup
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#lifecycle-rules
    - specs/tessariq-v0.1.0.md#tessariq-attach-run-ref
    - specs/tessariq-v0.1.0.md#tessariq-promote-run-ref
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-15T16:12:25Z"
areas:
    - lifecycle
    - workspace
    - attach
    - promote
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: The ENOENT-idempotent branch must be pinned at the unit level alongside the existing BUG-055 containment tests so a regression cannot silently re-break either direction.
    integration:
        required: false
        commands: []
        rationale: No new integration surface; the existing TASK-090 integration tests keep driving the real Docker chown/chmod/remove path and stay green unchanged.
    e2e:
        required: true
        commands:
            - go test -tags=e2e -run '^TestE2E_(AttachReconcilesExitedOrphanedRun|AttachForgedCrossRunEvidenceRejectsBeforeAttaching|PromoteTamperedManifestRejectedBeforeGitSideEffects)$' ./cmd/tessariq/...
        rationale: The three reconcile-adjacent e2e tests that TASK-090 exercised must still be green; they confirm `attach`/`promote` keep working end-to-end with real CLI binaries inside the runtime container.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*|.*_test\.go|metadata\.go|provision\.go|repoid\.go' --timeout-coefficient 10 --threshold-efficacy 70 ./internal/workspace
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*|.*_test\.go' --timeout-coefficient 10 --threshold-efficacy 70 ./internal/lifecycle
        rationale: New ENOENT branches in a containment predicate are exactly the kind of one-line change that mutation testing is good at catching — the branch must be killed by a deterministic test, not an incidental one.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check must prove that `tessariq attach <run-id>` succeeds after the worktree is manually removed and still fails when the stored `workspace_path` is genuinely tampered to point outside the canonical tree.
---

## Summary

Follow up on TASK-090 / BUG-055 with the regression flagged by Codex's PR #96 review: `workspace.ValidateWorkspacePath` rejects a missing canonical worktree instead of treating it as an idempotent no-op. `workspace.Cleanup` has always been safe to call on a missing path, so the new validator must preserve that contract. The fix is strictly additive on top of the BUG-055 containment envelope — it only short-circuits when the stored path is both lexically equal to the trusted-input canonical *and* does not exist on disk.

## Supersedes

- BUG-061 from `planning/BUGS.md`.

## Acceptance Criteria

- `workspace.ValidateWorkspacePath` returns `(canonical, nil)` when `workspace_path` matches the derived canonical lexically but the directory does not exist on disk (either the untrusted path itself or the recomputed canonical is missing).
- `lifecycle.cleanupTerminalRun` no longer errors out of `ReconcileRun` on a terminal non-success run whose worktree has already been removed; `workspace.Cleanup`'s `os.Stat + IsNotExist → nil` fast path runs as before.
- BUG-055 containment invariants stay intact:
    - Relative paths are still rejected before any filesystem call.
    - Lexical canonical equality against `workspace.WorkspacePath(homeDir, repoRoot, runID)` still rejects mismatches, including the *missing-path-outside-tree* case.
    - For paths that exist, symlink-leaf and symlink-ancestor escape rejection still fires via `filepath.EvalSymlinks` + `realWorktreesPrefix` containment.
    - `assertInsideWorktrees` inside `workspace.Cleanup` stays untouched as the defensive backstop.
- The existing `TestValidateWorkspacePath_NonexistentCanonicalRejected` is replaced with a positive idempotent test; a sibling test guards that a missing *lexically-outside* path is still rejected.
- A new lifecycle-level test drives reconcile through the `dependencies.cleanupWorkspace` hook with a terminal run whose canonical directory was never created, asserting `ReconcileRun` returns nil with `Live == false` and calls cleanup exactly once.
- Mutation testing on `internal/workspace` and `internal/lifecycle` still clears 70% with the new ENOENT branches killed.

## Test Expectations

- Start with a failing unit test (`TestValidateWorkspacePath_NonexistentCanonicalIsIdempotent`) that asserts the validator returns the canonical path with a nil error when the canonical directory does not exist.
- Add `TestValidateWorkspacePath_NonexistentOutsideTreePathStillRejected` so the ENOENT branch cannot short-circuit around the lexical canonical equality check for an attacker-supplied non-canonical path.
- Delete `TestValidateWorkspacePath_NonexistentCanonicalRejected` because it pins the regressed behavior.
- Add `TestReconcileRun_IdempotentWhenWorkspaceAlreadyRemoved` in `internal/lifecycle/reconcile_test.go` driving `reconcileRun` with a terminal status and a valid `workspace.json` pointing at a never-created canonical path, using a `cleanupWorkspace` spy.
- Re-run all existing BUG-055 regression tests (happy path, relative-path rejection, outside-tree rejection, wrong-run/repo IDs, symlink-leaf and symlink-ancestor escapes, tampered-workspace reconcile rejection, `workspace.Cleanup` defensive guards) without modification.
- Run mutation testing and confirm the ENOENT branches are killed.
- Manual test against a built CLI:
    - MT-001: plant a failed terminal run with valid canonical `workspace.json`, `rm -rf` the worktree, run `tessariq attach <run-id>`, verify exit 0 and no reconcile error.
    - MT-002: replay the TASK-090 decoy scenario with a tampered `workspace.json` pointing at `/tmp/tessariq-manual-test-TASK-095/decoy` and assert the BUG-055 rejection still fires and the decoy sentinel file is untouched.
    - MT-003: verify a successful run's worktree is still preserved on reconcile and a non-success run's worktree is still cleaned when the directory *does* exist.

## TDD Plan

1. RED: write the three new tests (two unit, one lifecycle). Confirm `TestValidateWorkspacePath_NonexistentCanonicalIsIdempotent` and `TestReconcileRun_IdempotentWhenWorkspaceAlreadyRemoved` fail on the current TASK-090 code and `TestValidateWorkspacePath_NonexistentOutsideTreePathStillRejected` passes (it already matches current behavior — keep it as a pin for post-fix regression protection).
2. GREEN: add the two `errors.Is(err, os.ErrNotExist)` short-circuit branches in `ValidateWorkspacePath` and update the doc comment to describe the new contract.
3. REFACTOR: none expected — the fix is a six-line addition inside the existing function.
4. VERIFY: run `gofmt`, `go vet`, targeted unit tests with `-race`, full `go test -race ./...`, integration suite, the three reconcile-adjacent e2e tests, mutation testing on `internal/workspace` and `internal/lifecycle`, and the manual test ladder.

## Notes

- Do NOT widen the ENOENT short-circuit to any path that is not already lexically equal to the trusted-input canonical — the lexical check is the security envelope, not an ergonomic sanity check.
- Do NOT change `assertInsideWorktrees` inside `workspace.Cleanup`; its non-existent-path fallback already enforces containment for the defensive backstop.
- Do NOT introduce a new error type; the `ErrWorkspacePathOutsideTree` sentinel stays the only failure surface for genuine containment violations.
- Push the fix as an additional commit on the existing PR #96 branch (`worktree-scalable-churning-pnueli`) addressing the Codex review comment; do not open a new PR.
- Codex review comment URL for evidence: `https://github.com/tessariq/tessariq/pull/96#discussion_r3087680616`.
- 2026-04-15T16:12:25Z: Fix BUG-061 (TASK-090 Codex P2 review): ValidateWorkspacePath ENOENT short-circuit keeps reconcile cleanup idempotent for missing canonical worktrees. Tests: 6 unit/lifecycle, integration workspace/lifecycle, e2e reconcile subset, gremlins 100%/86.11%, manual MT-001/MT-002/MT-003 all pass. PR #96 comment https://github.com/tessariq/tessariq/pull/96#discussion_r3087680616
