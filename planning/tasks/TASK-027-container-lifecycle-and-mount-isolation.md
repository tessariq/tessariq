---
id: TASK-027-container-lifecycle-and-mount-isolation
title: Implement Docker container lifecycle and mount isolation for agent execution
status: todo
priority: p1
depends_on:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
    - TASK-006-tmux-session-and-detached-attach-guidance
    - TASK-021-reference-runtime-image-and-docs
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-023-supported-agent-auth-mounts
    - TASK-024-claude-code-agent-runtime-integration
    - TASK-025-opencode-agent-runtime-integration
    - TASK-026-mount-agent-config-flag-and-config-dir-mounts
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#workspace-guarantees
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#evidence-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-03-31T12:00:00Z"
areas:
    - container
    - runtime
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Docker command construction and mount assembly are branch-heavy and must start with unit tests.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Real Docker container lifecycle requires Testcontainers-backed integration coverage.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Container isolation is a core user-visible security property and needs thin e2e coverage.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Mount assembly and container lifecycle are safety-critical.
    manual_test:
        required: true
        commands: []
        rationale: Real container isolation must be validated manually with a live agent run.
---

## Summary

Wrap agent execution in a Docker container with spec-required mount isolation, replacing the current direct host-exec model. The `runner.ProcessRunner` interface is the seam; agent packages become config builders and a new `internal/container/` package implements `ProcessRunner` via Docker CLI.

## Acceptance Criteria

- Agent binaries execute inside a Docker container, not directly on the host.
- The worktree is mounted read-write at `/work` inside the container.
- Evidence is mounted at a separate path inside the container (not under `/work`).
- Host `HOME` is never exposed inside the container.
- Auth files are mounted read-only at deterministic in-container paths under `/home/tessariq/`.
- Optional config directories are mounted read-only when `--mount-agent-config` is used.
- Environment variables (`CLAUDE_CONFIG_DIR`, future proxy vars) are injected via `docker create --env`.
- Container name matches `tessariq-<run_id>`.
- Container runs as the non-root `tessariq` user.
- Container is cleaned up (`docker rm -f`) on all exit paths (success, failure, timeout, interrupt).
- The tmux session stays on the host; container output is streamed to the session.
- Missing or unavailable Docker daemon fails before container creation with actionable guidance.
- `runtime.json` records the actual image used and mount metadata.
- Existing runner lifecycle (timeout, grace, pre/verify hooks, status.json) is unchanged.

## Test Expectations

- Add unit tests for Docker command construction: `docker create` args with mounts, env vars, working dir, container name, and user.
- Add unit tests for mount assembly from `authmount.MountSpec` to Docker `-v` flags.
- Add unit tests for signal mapping (SIGTERM to `docker stop`, SIGKILL to `docker kill`).
- Add integration tests for container create/start/wait/kill lifecycle using Testcontainers-backed Docker-in-Docker or equivalent.
- Add integration tests for container cleanup on error paths (process failure, timeout).
- Add integration tests for mount visibility: verify a file written inside `/work` in the container appears in the host worktree.
- Add a thin e2e test that runs the full pipeline and verifies the agent ran inside a container (not on host).
- Run mutation testing because mount assembly and lifecycle are safety-critical.

## TDD Plan

- Start with a failing unit test for Docker command construction (mount flags and env vars), then a failing integration test for container create/start/wait lifecycle.

## Notes

- The `ProcessRunner` interface is the clean seam; no changes to `runner.Runner` are needed.
- This task does not implement proxy networking (TASK-012 owns that); it provides the container that TASK-012 will attach to a Docker network.
- Agent-specific packages (`claudecode/`, `opencode/`) shift from ProcessRunner implementations to config builders (args, image, metadata).
