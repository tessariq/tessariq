---
id: TASK-044-spec-example-allowlist-source-consistency
title: Align manifest example allowlist_source with normative values
status: completed
priority: low
spec_ref: specs/tessariq-v0.1.0.md#evidence-contract
dependencies:
    - TASK-011-egress-mode-resolution-and-manifest-recording
updated_at: "2026-04-01T17:35:54Z"
---

## Summary

The minimum `manifest.json` example in the v0.1.0 spec currently uses `"allowlist_source": "auto"`, which conflicts with normative text restricting values to `cli`, `user_config`, or `built_in`.

## Supersedes

- BUG-012 from `planning/BUGS.md`.

## Acceptance Criteria

- The `manifest.json` example uses a normative value (`built_in`) for `allowlist_source`.
- No normative semantics are changed; only example consistency is corrected.
- Any nearby examples remain consistent with current implementation behavior.

## Test Expectations

- Run tracked-work validation to ensure spec refs remain valid.
- Regenerate or refresh any spec verification artifact only if required by workflow.

## TDD Plan

1. RED: identify inconsistency between example and normative value set.
2. GREEN: update the example value.
3. REFACTOR: scan nearby example fields for consistency.

## Notes

- This is intentionally a spec-doc task (not an implementation behavior change).
- 2026-04-01T17:35:54Z: Fixed allowlist_source in manifest.json example (line 409) and egress.compiled.yaml example (line 449) from 'auto' to 'built_in' to match normative text at line 317. Manual test: 3/3 pass. Verification: 0 findings.
