---
id: TASK-023-supported-agent-auth-mounts
title: Implement supported-agent auth discovery and read-only mounts
status: todo
priority: p0
depends_on:
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-021-reference-runtime-image-and-docs
    - TASK-022-agent-and-runtime-evidence-migration
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#product-intent
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-30T23:05:00Z"
areas:
    - agents
    - runtime
    - auth
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Per-agent detection rules and mount-policy decisions should start with unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Mount behavior and container-visible path expectations need real container validation.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Auth reuse is a core v0.1.0 UX promise and needs thin end-to-end coverage.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Detection and failure-UX branching should survive mutation testing.
    manual_test:
        required: true
        commands: []
        rationale: Real local auth reuse must be validated manually against the acceptance criteria.
---

## Summary

Implement the generic v0.1.0 auth-mount policy for supported agents: per-agent auth discovery, read-only mounts, no host `HOME` passthrough, and actionable failure UX.

## Acceptance Criteria

- Tessariq maintains per-agent knowledge of the supported auth files or directories required for each supported agent on Linux and macOS hosts.
- Claude Code required auth paths are exactly:
  - Linux: `~/.claude/.credentials.json` and `~/.claude.json`
  - macOS: `~/.claude/.credentials.json` when a file-backed credential mirror exists, and `~/.claude.json`
- OpenCode required auth paths are exactly `~/.local/share/opencode/auth.json` on Linux and macOS.
- Tessariq auto-detects the required supported-agent auth files or directories before agent start.
- Required supported-agent auth files or directories are mounted read-only into these deterministic in-container locations:
  - Claude Code: `$HOME/.claude/.credentials.json` and `$HOME/.claude.json`
  - OpenCode: `$HOME/.local/share/opencode/auth.json`
- Tessariq does not expose the host `HOME` directory inside the container.
- Tessariq fails cleanly when required supported-agent auth state is missing.
- Tessariq fails cleanly for Claude Code on macOS when only Keychain-backed auth exists and no file-backed credential mirror is present.
- Tessariq fails cleanly when the selected agent requires writable auth refresh or config mutation incompatible with the v0.1.0 contract.
- `runtime.json` records the read-only auth mount policy without recording secrets or host-home paths.
- Tessariq does not attempt direct macOS Keychain reuse for Claude Code in v0.1.0.

## Test Expectations

- Add unit tests for per-agent auth discovery on Linux and macOS path layouts, missing-auth detection, and no-HOME policy enforcement.
- Add unit tests for the macOS Claude Code credential-mirror requirement.
- Add integration tests that the expected read-only mounts are present inside the container for supported agents at the documented deterministic destinations.
- Add integration tests that writable refresh expectations are rejected cleanly.
- Add thin e2e coverage for the success path and missing-auth failure path.
- Run mutation testing because the detection and guardrail logic are safety-critical.

## TDD Plan

- Start with a failing unit test for missing-auth detection and a failing integration test for read-only auth mounts.

## Notes

- Agent-specific command and option integration remains in the dedicated supported-agent tasks.
