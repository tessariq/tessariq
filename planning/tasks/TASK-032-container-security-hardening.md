---
id: TASK-032-container-security-hardening
title: Add container capability dropping, privilege escalation prevention, and evidence permission hardening
status: done
priority: p0
depends_on:
    - TASK-027-container-lifecycle-and-mount-isolation
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#evidence-permissions
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-03-31T18:00:37Z"
areas:
    - container
    - evidence
    - security
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Container arg construction and evidence file creation are unit-testable.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: Container security flags must be verified against real Docker.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: Agent must still function correctly with dropped capabilities.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Security flag injection is easy to accidentally weaken.
    manual_test:
        required: true
        commands: []
        rationale: Container security posture should be manually verified via docker inspect.
---

## Summary

The v0.1.0 spec now requires agent containers to drop all Linux capabilities and prevent privilege escalation. Evidence files must also be owner-only accessible. This task implements both requirements.

## Acceptance Criteria

### Container hardening

- Agent containers are created with `--cap-drop=ALL`.
- Agent containers are created with `--security-opt=no-new-privileges`.
- The hardening flags are present in the `docker create` argument list for every run.
- The agent (Claude Code and OpenCode) continues to function correctly inside containers with dropped capabilities.
- Container hardening flags do not apply to workspace repair containers (which need root for `chown`).

### Evidence file permissions

- Evidence directories are created with `0o700` (owner-only access).
- Evidence files are created with `0o600` (owner-only read/write).
- All evidence-writing code paths use the restricted permissions: `manifest.json`, `status.json`, `agent.json`, `runtime.json`, `workspace.json`, `task.md`, `run.log`, `runner.log`, `egress.compiled.yaml`, and `index.jsonl`.
- Existing evidence from prior runs is not retroactively modified.

## Test Expectations

- Add unit tests verifying `--cap-drop=ALL` and `--security-opt=no-new-privileges` appear in the `docker create` argument list.
- Add unit tests verifying evidence directories are created with `0o700` and files with `0o600`.
- Add integration tests confirming the agent process starts and runs successfully with dropped capabilities.
- Add integration tests verifying evidence file permissions on disk after a run completes.
- Add a thin e2e test confirming a full run succeeds with the hardened container configuration.
- Run mutation testing because security flag injection is safety-critical.

## TDD Plan

1. RED: write unit test asserting `--cap-drop=ALL` and `--security-opt=no-new-privileges` appear in `docker create` args.
2. RED: write unit test asserting evidence directories are created with `0o700`.
3. RED: write unit test asserting evidence files are created with `0o600`.
4. GREEN: add security flags to container create arg builder.
5. GREEN: update evidence directory and file creation to use restricted permissions.
6. IMPROVE: ensure repair containers are excluded from capability dropping.
7. RED: write integration tests confirming agent starts with dropped capabilities.
8. RED: write integration tests verifying evidence file permissions on disk.
9. GREEN: verify integration tests pass.
10. RED: write thin e2e test confirming a full run succeeds with hardened config.
11. GREEN: verify e2e test passes.

## Notes

- Files likely affected: `internal/container/process.go` (`buildCreateArgs`), `internal/container/config.go`, `internal/run/manifest.go`, `internal/run/taskcopy.go`, `internal/runner/status.go`, `internal/runner/logs.go`, and evidence writing functions across `internal/run/` and `internal/runner/`.
- Repair containers need root for `chown` and must NOT get `--cap-drop=ALL`.
- 2026-03-31T18:00:37Z: Container hardening: --cap-drop=ALL, --security-opt=no-new-privileges added to buildCreateArgs. Evidence permissions: dirs 0o700, files 0o600. Tests: 2 unit + 2 integration (container), 11 unit (permissions), 1 e2e. Mutation testing: 83.48% efficacy. Manual test: planning/artifacts/manual-test/TASK-032-container-security-hardening/20260331T175937Z/
