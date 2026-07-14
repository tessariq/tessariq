---
id: TASK-063-pin-squid-proxy-image-by-digest
title: Pin the default Squid proxy image by digest
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#networking-and-egress
dependencies:
    - TASK-012-proxy-topology-and-egress-artifacts
updated_at: "2026-04-03T11:23:22Z"
---

## Summary

`DefaultSquidImage` still points to `ubuntu/squid:latest`, which leaves proxy-mode runs exposed to mutable-tag drift and supply-chain compromise. Pin the default image by digest the same way the workspace repair image is already pinned.

## Supersedes

- BUG-030 from `planning/BUGS.md`.

## Acceptance Criteria

- `DefaultSquidImage` uses an immutable digest reference.
- The pinned image remains the default when no explicit Squid image override is supplied.
- Proxy integration coverage passes with the pinned digest.
- The digest is stored in a well-named constant that is easy to update during maintenance.

## Test Expectations

- Add unit coverage proving the default image contains `@sha256:`.
- Add regression coverage that `Topology` and `StartSquid` still default to the pinned image.
- Run proxy integration coverage against the pinned image.

## TDD Plan

1. RED: add a test asserting `DefaultSquidImage` is digest-pinned.
2. GREEN: replace the mutable tag with an immutable digest.
3. GREEN: rerun proxy startup coverage to verify behavior is unchanged.

## Notes

- Likely files: `internal/proxy/squid.go`, `internal/proxy/topology.go`, and proxy tests.
- Follow the same maintenance pattern already used for `internal/workspace/provision.go`'s repair image.
- 2026-04-03T11:23:22Z: Pinned DefaultSquidImage by digest; all unit/integration tests pass; manual test 4/4 pass; zero verification findings
