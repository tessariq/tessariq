---
id: TASK-088-require-proxy-evidence-completeness-before-promote
title: Require proxy evidence completeness before promote
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#networking-and-egress
dependencies:
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-015-promote-branch-commit-trailers-and-zero-diff-guard
    - TASK-047-promote-repo-local-evidence-path-validation
updated_at: "2026-04-14T14:41:09Z"
---

## Summary

Make proxy-mode evidence completeness a real promote gate so runs recorded as `resolved_egress_mode=proxy` cannot be promoted without their required egress artifacts.

## Supersedes

- BUG-051 from `planning/BUGS.md`.

## Acceptance Criteria

- When `manifest.json.resolved_egress_mode` is `proxy`, evidence completeness checks require non-empty `egress.compiled.yaml` and `egress.events.jsonl` in addition to the unconditional artifact set.
- `tessariq promote` fails cleanly with missing-evidence guidance when a proxy-mode run is missing either required egress artifact.
- Non-proxy runs are unaffected and do not require proxy-only evidence files.
- The implementation uses one clear source of truth for mode-aware completeness so promote-time rules do not drift from the evidence contract.

## Test Expectations

- Add unit tests covering proxy and non-proxy completeness decisions from real manifest values.
- Add integration or promote-level tests proving that proxy-mode runs missing `egress.compiled.yaml` or `egress.events.jsonl` are rejected.
- Add end-to-end coverage for at least one proxy-mode run that promotes successfully only when both egress artifacts are intact.
- Run mutation testing because conditional completeness checks are easy to weaken accidentally.

## TDD Plan

1. RED: add failing completeness coverage for a proxy-mode run missing one of the required egress artifacts.
2. GREEN: implement the smallest mode-aware completeness gate.
3. GREEN: ensure promote surfaces the missing-artifact failure in the same contract style as other evidence gaps.
4. VERIFY: rerun proxy-mode automated and manual checks.

## Notes

- Keep `squid.log` optional, matching the spec; this task is only about the two required proxy-mode artifacts.
- Reuse or extend existing completeness helpers instead of scattering another bespoke artifact check in parallel code paths.
- 2026-04-14T14:41:09Z: proxy completeness gate enforced at runner.CheckEvidenceCompleteness; unit+integration+e2e+mutation+manual tests green
