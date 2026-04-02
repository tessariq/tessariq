---
id: TASK-063-pin-squid-proxy-image-by-digest
title: Pin the default Squid proxy image by digest
status: todo
priority: p1
depends_on:
    - TASK-012-proxy-topology-and-egress-artifacts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-02T14:59:17Z"
areas:
    - proxy
    - security
    - supply-chain
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Image-reference pinning is deterministic constant-level behavior.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The pinned image still needs to start cleanly in proxy-mode tests.
    e2e:
        required: false
        commands: []
        rationale: Existing proxy runtime coverage plus integration startup checks are sufficient.
    mutation:
        required: false
        commands: []
        rationale: This is constant replacement rather than branch-heavy logic.
    manual_test:
        required: false
        commands: []
        rationale: Digest pinning is fully verifiable through automated checks and inspection.
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
