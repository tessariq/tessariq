---
id: TASK-096-make-proxy-telemetry-extraction-fail-closed
title: Fail closed when proxy teardown telemetry extraction cannot produce trustworthy evidence
status: todo
priority: p1
depends_on:
    - TASK-012-proxy-topology-and-egress-artifacts
    - TASK-067-cleanup-squid-resources-on-startup-failure
    - TASK-088-require-proxy-evidence-completeness-before-promote
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-24T07:41:10Z"
areas:
    - proxy
    - evidence
    - reliability
    - promote
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Teardown error-shaping and evidence-write decisions are deterministic and should be pinned first at unit level.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The bug sits at the real Docker and Squid teardown boundary, so integration coverage must exercise actual proxy log extraction failure paths.
    e2e:
        required: false
        commands: []
        rationale: Focused proxy integration coverage is sufficient if it proves promote rejects untrustworthy proxy evidence.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Best-effort cleanup branches are easy to weaken into silent evidence corruption while keeping happy-path tests green.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check should confirm that proxy-mode runs do not leave behind misleading empty telemetry artifacts when Squid log extraction fails.
---

## Summary

`Topology.Teardown` currently ignores failures from `CopyAccessLog`, `ParseSquidAccessLog`, `WriteEventsJSONL`, and `CopySquidLog`, then proceeds to write empty-looking proxy evidence as if no blocked destinations occurred. Harden proxy teardown so Tessariq either emits valid telemetry artifacts or records a clear non-success or incomplete-evidence outcome instead of silently fabricating a clean result.

## Supersedes

- BUG-052 from `planning/BUGS.md`.

## Acceptance Criteria

- If proxy teardown cannot extract or parse Squid access-log data, Tessariq does not silently write misleading "zero blocked destinations" evidence.
- Proxy-mode evidence distinguishes "no denied events occurred" from "telemetry extraction failed".
- `egress.events.jsonl` and related proxy evidence are either trustworthy artifacts derived from real log data or the run is marked incomplete or non-success in a way that prevents silent promote.
- Infrastructure cleanup of the Squid container and Docker network still runs even when telemetry extraction fails.
- Failure messaging preserves the root telemetry-extraction cause and does not mask it behind cleanup noise.
- `tessariq promote` cannot promote a proxy-mode run whose telemetry evidence is missing, synthetic, or known-untrustworthy.

## Test Expectations

- Start with a failing unit test showing that teardown currently swallows a `CopyAccessLog` failure and still writes empty-looking evidence.
- Add unit coverage for parse failures and artifact-write failures so each branch has one explicit expected outcome.
- Add integration coverage that forces access-log extraction failure against a real proxy container and verifies cleanup still happens while promote-facing evidence is not silently forged.
- Add promote-level coverage proving a proxy-mode run with failed telemetry extraction is rejected as incomplete or failed, matching the chosen contract.
- Run mutation testing because the failure-handling branches are security- and audit-sensitive.

## TDD Plan

1. RED: reproduce the current swallowed-error path for `CopyAccessLog` and parse failure.
2. GREEN: choose one fail-closed contract and implement it consistently in teardown and completeness or promotion flow.
3. GREEN: preserve container and network cleanup even when evidence extraction fails.
4. VERIFY: rerun proxy integration coverage, promote completeness coverage, and manual proxy teardown testing.

## Notes

- Keep this task focused on trustworthy proxy telemetry evidence; do not mix it with unrelated allowlist or startup-topology work.
- A valid implementation may surface a teardown error directly, record an explicit extraction-failure marker, or withhold required artifacts so completeness fails, but it must not fabricate a clean empty result.
- Add this task to `TASK-017-v0-1-0-spec-conformity-closeout` dependencies so the release gate cannot run before BUG-052 is resolved.
