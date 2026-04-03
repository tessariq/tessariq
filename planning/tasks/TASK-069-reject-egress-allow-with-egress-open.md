---
id: TASK-069-reject-egress-allow-with-egress-open
title: Reject misleading --egress open plus --egress-allow combinations
status: done
priority: p2
depends_on:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-053-bypass-user-config-when-cli-egress-is-explicit
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-03T08:59:22Z"
areas:
    - cli
    - networking
    - validation
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Flag-validation behavior is deterministic CLI logic.
    integration:
        required: false
        commands: []
        rationale: The fix should fail at config validation time before runtime collaborators matter.
    e2e:
        required: false
        commands: []
        rationale: Unit and command-level coverage are sufficient for this validation rule.
    mutation:
        required: false
        commands: []
        rationale: This is a narrow validation branch rather than complex business logic.
    manual_test:
        required: true
        commands: []
        rationale: Confirms the CLI stops users from believing open mode still enforces an allowlist.
---

## Summary

`--egress open` starts no proxy, so any supplied `--egress-allow` values are ignored. Reject that combination at validation time so users cannot believe they requested restricted egress while actually getting unrestricted networking.

## Supersedes

- BUG-036 from `planning/BUGS.md`.

## Acceptance Criteria

- `tessariq run --egress open --egress-allow ...` fails fast with an actionable validation error.
- Existing rejection for `--egress none --egress-allow ...` remains unchanged.
- Open mode without allowlist entries continues to work unchanged.
- Error messaging makes clear that allowlists only apply to proxy-based egress.

## Test Expectations

- Add unit tests for the rejected flag combination.
- Add regression coverage for valid `open`, `proxy`, and `none` combinations.

## TDD Plan

1. RED: add a failing config-validation test for `open` plus `egress-allow`.
2. GREEN: reject the misleading flag combination.
3. GREEN: keep valid egress combinations unchanged.

## Notes

- Likely files: `internal/run/config.go` and `internal/run/config_test.go`.
- A hard validation error is preferred over a warning because the current behavior silently weakens the user's intended restriction.
- 2026-04-03T08:59:22Z: Validation rejects egress open + egress-allow; unit tests cover open, unsafe-egress, and regression paths; manual test 4/4 pass
