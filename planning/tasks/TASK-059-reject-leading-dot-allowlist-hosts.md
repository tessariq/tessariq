---
id: TASK-059-reject-leading-dot-allowlist-hosts
title: Reject leading-dot allowlist hosts that widen Squid matching
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#networking-and-egress
dependencies:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-012-proxy-topology-and-egress-artifacts
updated_at: "2026-04-03T10:28:46Z"
---

## Summary

Squid treats `.example.com` in `dstdomain` ACLs as a wildcard matcher for subdomains, but the spec promises host:port granularity. Reject leading-dot hosts during allowlist parsing so Tessariq never widens a single host entry into a domain wildcard.

## Supersedes

- BUG-026 from `planning/BUGS.md`.

## Acceptance Criteria

- `ParseDestination` rejects hosts that begin with `.`.
- CLI and user-config allowlist inputs fail with the same validation error.
- Generated Squid configs cannot contain leading-dot `dstdomain` ACL entries.
- Canonical hostnames without a leading dot continue to work unchanged.

## Test Expectations

- Add unit tests covering leading-dot rejection and ordinary host acceptance.
- Add integration coverage proving a proxy-mode run cannot compile a wildcard-style host entry.

## TDD Plan

1. RED: add a failing parser test for `.github.com`.
2. GREEN: reject leading-dot hosts before normalization.
3. GREEN: keep existing host-only and host:port cases passing.

## Notes

- Likely files: `internal/run/allowlist.go` and `internal/run/allowlist_test.go`.
- Keep the rejection in the shared parser rather than in Squid-config generation so all allowlist sources behave identically.
- 2026-04-03T10:28:46Z: ParseDestination rejects leading-dot hosts; unit, integration, mutation, and manual tests pass
