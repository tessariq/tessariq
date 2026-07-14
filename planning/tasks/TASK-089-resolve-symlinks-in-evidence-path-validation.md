---
id: TASK-089-resolve-symlinks-in-evidence-path-validation
title: Resolve symlinks in evidence path validation
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#core-workflow
dependencies:
    - TASK-047-promote-repo-local-evidence-path-validation
    - TASK-051-attach-repo-local-evidence-path-validation
    - TASK-052-attach-run-id-evidence-path-consistency
updated_at: "2026-04-14T16:27:24Z"
---

## Summary

Close the remaining evidence-path escape by validating real filesystem targets, not only cleaned path strings, before attach or promote trusts run evidence.

## Supersedes

- BUG-054 from `planning/BUGS.md`.

## Acceptance Criteria

- `ValidateEvidencePath` resolves symlinks for both the repository root and the candidate evidence path before enforcing repo-local containment.
- `tessariq attach` and `tessariq promote` reject evidence directories whose real target escapes the repository's `.tessariq/runs/` tree, even when the lexical path stays under that prefix.
- Validation still enforces the existing run-id-to-directory-name consistency checks after symlink resolution.
- Tests cover both a symlinked leaf evidence directory and at least one intermediate symlink component.

## Test Expectations

- Start with failing unit tests showing that a symlink under `.tessariq/runs/` currently passes validation.
- Add attach and promote coverage proving those commands reject symlink-forged evidence.
- Add end-to-end or high-level coverage for one realistic forged-index scenario using a symlink target outside the repository.
- Run mutation testing because containment checks are security-sensitive and easy to overfit to a single path shape.

## TDD Plan

1. RED: reproduce the current symlink escape against `ValidateEvidencePath`.
2. GREEN: switch containment checks to real-path validation while preserving existing `run_id` consistency guarantees.
3. GREEN: prove attach and promote both consume the hardened validation path.
4. VERIFY: rerun attach/promote security coverage and manual validation.

## Notes

- Mirror the stricter approach already used for task-path symlink validation instead of inventing a second containment model.
- Keep this task focused on evidence-path containment; do not mix it with unrelated proxy or index-shape work.
- 2026-04-14T16:27:24Z: hardened ValidateEvidencePath with filepath.EvalSymlinks mirroring ValidateTaskPath; added unit + integration coverage for leaf and intermediate symlink escapes; attach + promote unit tests reject forged symlink evidence before status read or tmux session; gremlins efficacy 100% on evidencepath.go mutations; manual test MT-001..MT-004 all pass with forged symlinks rejected and legitimate repo-local runs still accepted
