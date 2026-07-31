---
id: T-120-correct-t1-6-post-run-hygiene-expectations-for
title: Correct T1.6 post-run hygiene expectations for retained worktrees
status: todo
priority: low
spec_ref: specs/tessariq-v0.1.0.md#workspace-guarantees
dependencies:
    - T-116
updated_at: "2026-07-31T11:51:42Z"
---

# T-120-correct-t1-6-post-run-hygiene-expectations-for Correct T1.6 post-run hygiene expectations for retained worktrees

## Description

The manual release-readiness sweep case T1.6 ("post-run hygiene") tells the tester
to expect `ls ~/.tessariq/worktrees/*/` to show "worktree for this run removed" and
`git worktree list` to show "no stale entries" after a normal run. That expectation
is wrong for a **successful** run.

`cmd/tessariq/run.go:411` sets `cleanupWorktree = false` once the run succeeds,
because the worktree is the workspace `tessariq promote` commits from. `promote`
does not remove it either, and `specs/tessariq-v0.2.0.md:55` records that a
first-class `clean`/prune command is deliberately deferred past v0.2.0. A retained
worktree after a successful run is therefore intended behavior, not a leak.

A tester following T1.6 as written would raise a false release blocker. The other
four resource classes in T1.6 (containers, networks, runtime-state, git worktree
staleness on *non-success* runs) are correct as written.

Discovered while implementing T-116, whose own acceptance criteria carried the same
wrong expectation; the e2e coverage added there pins the real behavior via
`requireWorktreeRetained` (success) and `requireWorktreeRemoved` (timeout,
interrupt, failure).

## Acceptance

- The T1.6 sweep in `docs/release-readiness-v0.1.0.md` distinguishes the success
  path (worktree and its `git worktree list` entry are retained on purpose, as the
  promotable workspace) from non-success paths (worktree and entry removed).
- The corrected text names the reason for retention so a tester does not "fix" it,
  and points at the deferred-`clean` note in `specs/tessariq-v0.2.0.md`.
- No production behavior change: this is a documentation correction only.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- `docs/release-readiness-v0.1.0.md` was an untracked working-tree draft at the time
  of discovery; confirm its tracked state before editing.
- Automated coverage already exists and needs no change:
  `TestE2E_SuccessfulRunLeavesNoResources` and `TestE2E_TimedOutRunLeavesNoResources`
  in `cmd/tessariq/run_e2e_test.go`.
