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
    - specs/tessariq-v0.1.0.md#core-workflow
    - specs/tessariq-v0.1.0.md#repository-model
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#failure-ux
    - specs/tessariq-v0.1.0.md#evidence-contract
updated_at: "2026-03-29T12:06:20Z"
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
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Defaulting and option-application logic are mutation-prone and should meet the CI threshold once implemented.
    manual_test:
        required: true
        rationale: Validates CLI behavior and evidence artifacts through direct execution against acceptance criteria.
---

## Summary

Add `tessariq run <task-path>` command wiring, supported flags, task-path validation, `--attach` handling, and manifest bootstrap scaffolding.

## Acceptance Criteria

- All v0.1.0 run flags parse with the documented defaults: `--timeout=30m`, `--grace=30s`, `--agent=claude-code`, `--egress=auto`, and `--attach=false`.
- The supported flag surface is decision-complete for v0.1.0: `--agent`, `--image`, `--model`, `--yolo`, `--egress`, `--unsafe-egress`, `--egress-allow`, `--egress-allow-reset`, `--pre`, `--verify`, and `--attach`.
- `--unsafe-egress` behaves exactly as an alias for `--egress open`, and repeatable flags preserve CLI order for later allowlist processing.
- Invalid flag combinations fail cleanly before execution.
- Missing paths, non-Markdown paths, and paths outside the current repository fail before container start with the required guidance.
- `--attach` is wired as the non-default live-attach path while detached mode remains the default UX.
- Manifest bootstrap exists before long-running work starts and records the stable fields available at command-preflight time: `schema_version`, `run_id`, `task_path`, `adapter`, `requested_egress_mode`, and `created_at`.
- Follow-on tasks explicitly own filling the remaining required manifest fields before runner bootstrap begins: `task_title` in `TASK-003`, `base_sha` and `workspace_mode` in `TASK-004`, `container_name` in `TASK-005`, and `resolved_egress_mode` plus `allowlist_source` in `TASK-011`.

## Test Expectations

- Add unit tests for default values, aliases, invalid task-path rejection, `--attach` handling, and manifest bootstrap content.
- Add unit tests for ULID format validation of generated `run_id` values.
- Add boundary-value unit tests for temporal flags: `--timeout=0`, `--timeout=-1`, `--grace=0`, `--grace` larger than `--timeout`, and absurdly large values.
- Add unit tests for contradictory flag combinations: `--egress none` with `--egress-allow`, `--egress-allow` without proxy mode, and `--pre`/`--verify` with empty strings.
- Integration tests are deferred until a real container lifecycle exists and must then use Testcontainers only.
- Error-path e2e tests for invalid task-path failure are consolidated in `TASK-017` closeout sweep.
- Run mutation testing because flag/default logic is non-trivial.

## TDD Plan

- Start with a failing unit test for the defaulted run configuration and manifest seed values.

## Notes

- Keep printed output and preflight failures aligned with the spec once the run flow becomes executable.
