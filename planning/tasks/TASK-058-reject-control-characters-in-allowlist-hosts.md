---
id: TASK-058-reject-control-characters-in-allowlist-hosts
title: Reject control characters in allowlist hosts before Squid config generation
status: done
priority: p0
depends_on:
    - TASK-011-egress-mode-resolution-and-manifest-recording
    - TASK-012-proxy-topology-and-egress-artifacts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-03T09:04:09Z"
areas:
    - networking
    - proxy
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Destination parsing and Squid-conf generation are deterministic string-handling logic.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Proxy-mode validation should be exercised with real Squid config generation paths.
    e2e:
        required: false
        commands: []
        rationale: Integration coverage is sufficient once invalid input is rejected before runtime.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Input-validation branches are security-sensitive and easy to weaken accidentally.
    manual_test:
        required: true
        commands: []
        rationale: Confirms malformed user-config and CLI allowlist values are rejected with actionable errors.
---

## Summary

`ParseDestination` rejects only spaces and tabs today, so newline and other control bytes can flow into `GenerateSquidConf` and inject standalone Squid directives. Reject control characters before any host is normalized or interpolated into proxy config.

## Supersedes

- BUG-025 from `planning/BUGS.md`.

## Acceptance Criteria

- Allowlist hosts containing `\n`, `\r`, `\t`, NUL, or any other ASCII control byte are rejected.
- Rejected destinations fail before `egress.compiled.yaml` or Squid config generation.
- Error messaging identifies the host as invalid input rather than surfacing a downstream Squid failure.
- Valid hostname inputs continue to normalize exactly as before.

## Test Expectations

- Add unit tests for newline, carriage-return, tab, and other control-character inputs.
- Add regression coverage proving ordinary hostnames still parse successfully.
- Add integration coverage proving proxy setup never emits injected directives from malformed config.

## TDD Plan

1. RED: add parsing tests for control-character hosts.
2. GREEN: tighten host validation in `ParseDestination` before config generation.
3. REFACTOR: keep validation centralized so CLI and user-config paths share the same enforcement.

## Notes

- Likely files: `internal/run/allowlist.go`, `internal/run/allowlist_test.go`, and proxy integration coverage.
- Prefer rejecting all bytes `< 0x20` plus `0x7f` instead of trying to special-case only newline variants.
- 2026-04-03T09:04:09Z: Tightened ParseDestination to reject all ASCII control bytes (0x00-0x1F, 0x7F) and space. Unit tests, integration tests, mutation testing (85.9% efficacy), and manual testing all pass.
