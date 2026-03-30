---
id: TASK-021-reference-runtime-image-and-docs
title: Publish the v0.1.0 minimal reference runtime image and docs
status: todo
priority: p0
depends_on:
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-002-run-cli-flags-and-manifest-bootstrap
    - TASK-020-prerequisite-preflight-and-missing-dependency-ux
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#product-intent
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#specification-changelog
updated_at: "2026-03-30T23:05:00Z"
areas:
    - runtime
    - docker
    - docs
verification:
    unit:
        required: false
        commands:
            - go test ./...
        rationale: Most work is image and documentation oriented rather than core branch logic.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The runtime image contract should be validated through real container behavior.
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: End-to-end agent flows are owned by the agent integration tasks.
    mutation:
        required: false
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: This task is not primarily branch-heavy logic.
    manual_test:
        required: true
        commands: []
        rationale: Runtime-image usability and docs need direct verification.
---

## Summary

Publish the official minimal Tessariq reference runtime image for v0.1.0 and document how users derive a compatible runtime image that also contains their chosen agent binary.

## Acceptance Criteria

- Tessariq publishes the official minimal reference runtime image as `ghcr.io/tessariq/reference-runtime:v0.1.0`.
- The reference runtime image is built from `debian:bookworm-slim` or an equivalent glibc-based base image and uses a non-root default user.
- The reference runtime image includes the documented baseline tools required by the active v0.1.0 spec: `bash`, `ca-certificates`, `curl`, `git`, `jq`, `ripgrep`, `zip`, `unzip`, `tar`, `xz-utils`, `patch`, `procps`, `less`, `openssh-client`, `make`, `build-essential`, `pkg-config`, Python 3 with `pip` and `venv`, Node LTS with `npm` and `corepack`, and Go `1.26`.
- The reference runtime image does not bundle Claude Code, OpenCode, or other third-party agent binaries.
- The reference runtime image uses a versioned tag and the task does not introduce `latest` as a release contract.
- Documentation explains that Tessariq reuses supported auth state inside a compatible runtime image and does not reuse the host-installed binary.
- Documentation explains how to derive or choose a compatible runtime image when the selected agent binary is not present in the reference runtime image.
- Documentation includes a derived-image example that installs a supported agent binary into the reference runtime.
- Documentation includes an informative future note about the macOS Claude Code Keychain host-helper pattern without making it part of the supported v0.1.0 contract.

## Test Expectations

- Add integration coverage that the reference runtime image starts successfully and contains the expected baseline tools.
- Add integration coverage that the reference runtime image runs as a non-root user.
- Add integration checks that the reference runtime image does not contain the supported third-party agent binaries by default.
- Manual test the documented derivation flow using a compatible runtime image containing a supported agent binary.

## TDD Plan

- Start with a failing integration test or image-inspection check for the required baseline toolchain.

## Notes

- This task is intentionally about the safe baseline runtime and its documentation, not upstream third-party agent version tracking.
