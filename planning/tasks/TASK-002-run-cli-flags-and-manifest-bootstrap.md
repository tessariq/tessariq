---
id: TASK-002-run-cli-flags-and-manifest-bootstrap
title: Add run command flags and manifest bootstrap
status: todo
priority: p0
depends_on:
  - TASK-001-init-skeleton-and-gitignore
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
  - specs/tessariq-v0.1.0.md#product-intent
  - specs/tessariq-v0.1.0.md#cli-run
updated_at: 2026-03-29T00:00:00Z
areas:
  - cli
  - evidence
verification:
  unit:
    required: true
    commands:
      - go test ./...
    rationale: Flag parsing, defaulting, and manifest bootstrap should be covered through focused unit tests.
  integration:
    required: false
    commands:
      - go test -tags=integration ./...
    rationale: Containerized integration coverage becomes useful once the run pipeline starts external collaborators.
  e2e:
    required: false
    commands:
      - go test -tags=e2e ./...
    rationale: The user-visible run flow is not complete until runner and adapter tasks land.
  mutation:
    required: true
    commands:
      - "gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"
    rationale: Defaulting and option-application logic are mutation-prone and should meet the CI threshold once implemented.
---

## Summary

Add `tessariq run <task-path>` command wiring, supported flags, and initial manifest creation.

## Acceptance Criteria

- All v0.1.0 run flags parse with the documented defaults.
- Invalid flag combinations fail cleanly before execution.
- Initial manifest data exists before long-running work starts.

## Test Expectations

- Add unit tests for default values, aliases, and manifest bootstrap content.
- Integration tests are deferred until a real container lifecycle exists and must then use Testcontainers only.
- E2E coverage is deferred until the full run flow is stitched together.
- Run mutation testing because flag/default logic is non-trivial.

## TDD Plan

- Start with a failing unit test for the defaulted run configuration and manifest seed values.

## Notes

- Keep printed output aligned with the spec once the run flow becomes executable.
