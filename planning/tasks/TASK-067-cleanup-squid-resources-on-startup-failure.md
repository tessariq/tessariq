---
id: TASK-067-cleanup-squid-resources-on-startup-failure
title: Clean up Squid containers and networks when proxy startup fails mid-sequence
status: done
priority: p1
depends_on:
    - TASK-012-proxy-topology-and-egress-artifacts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#networking-and-egress
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-03T11:27:04Z"
areas:
    - proxy
    - cleanup
    - reliability
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Partial-start cleanup control flow should be covered deterministically first.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Real Docker resources are needed to prove failed startup does not leak containers or networks.
    e2e:
        required: false
        commands: []
        rationale: Focused proxy integration coverage is sufficient for this failure path.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Error-path cleanup branches are subtle and easy to regress.
    manual_test:
        required: true
        commands: []
        rationale: Confirms a broken proxy startup leaves no orphaned Docker resources behind.
---

## Summary

`StartSquid` creates Docker resources in stages but does not tear them down when later steps fail, and `Topology.Setup` only removes the network on failure. Add best-effort cleanup so partial proxy startup cannot orphan a Squid container or the network attached to it.

## Supersedes

- BUG-034 from `planning/BUGS.md`.

## Acceptance Criteria

- If `docker cp`, `network connect`, `docker start`, or readiness checks fail, the created Squid container is removed.
- The per-run Docker network is also removed successfully on startup failure.
- Cleanup remains idempotent if some resources were never created or are already gone.
- Failure messaging still reports the startup root cause rather than masking it with cleanup noise.

## Test Expectations

- Add unit tests for cleanup on each post-create failure branch.
- Add integration coverage that forces a failed startup and verifies no `tessariq-squid-*` container or network remains.
- Add regression coverage for successful proxy startup and teardown.

## TDD Plan

1. RED: add a failing startup test that leaves an orphaned Squid resource.
2. GREEN: add best-effort container cleanup to the failing startup path.
3. GREEN: ensure network cleanup runs after container cleanup so it can succeed.

## Notes

- Likely files: `internal/proxy/squid.go`, `internal/proxy/topology.go`, and proxy integration tests.
- Keep cleanup local to the failing startup path so successful runs keep the existing teardown behavior.
- 2026-04-03T11:27:04Z: Added deferred container cleanup in StartSquid and belt-and-suspenders StopSquid call in Topology.Setup. Integration test verifies no orphaned resources after startup failure. Manual test confirms all 4 acceptance criteria. Mutation testing at 89.23% efficacy.
