---
id: TASK-065-support-ipv6-safe-egress-destination-parsing
title: Parse IPv6 egress destinations without corrupting host and port values
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#networking-and-egress
dependencies:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-012-proxy-topology-and-egress-artifacts
updated_at: "2026-04-03T11:27:01Z"
---

## Summary

`ParseDestination` splits on the last colon, which corrupts IPv6 addresses and bracketed host-port forms. Replace the parser with IPv6-aware handling so Tessariq either accepts supported IPv6 destination syntax correctly or rejects it explicitly and safely.

## Supersedes

- BUG-032 from `planning/BUGS.md`.

## Acceptance Criteria

- Bracketed IPv6 host-port forms parse into the correct host and port.
- Bare hosts without an explicit port still default to 443 only when the input is unambiguous.
- Invalid or unsupported IPv6 forms fail with actionable validation errors rather than silently producing corrupted hosts.
- Downstream compiled allowlist and Squid config generation receive normalized host and port values.

## Test Expectations

- Add unit tests covering bracketed IPv6, malformed IPv6, host-only forms, and ordinary hostname regressions.
- Add integration coverage that compiled destinations stay well-formed for accepted IPv6 inputs.

## TDD Plan

1. RED: add a parser matrix that exposes current IPv6 misparsing.
2. GREEN: switch to IPv6-aware splitting and normalization.
3. GREEN: keep existing hostname parsing behavior stable.

## Notes

- Likely files: `internal/run/allowlist.go` and `internal/run/allowlist_test.go`.
- Be explicit about which IPv6 forms are supported; rejecting ambiguous bare forms is preferable to silently misparsing them.
- 2026-04-03T11:27:01Z: IPv6-safe ParseDestination: bracketed forms parsed via net.SplitHostPort, bare IPv6 rejected with actionable error. Unit tests, integration tests, mutation testing (85.79%), and manual tests all pass.
