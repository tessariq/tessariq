---
id: TASK-055-version-command-and-root-version-flag
title: Add version command and root version flag
status: done
priority: p1
depends_on: []
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-version
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: "2026-04-01T20:21:40Z"
areas:
    - cli
    - versioning
    - docs
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Root-command wiring and output formatting are deterministic and should be covered with focused unit tests.
    integration:
        required: false
        commands: []
        rationale: The feature is pure CLI metadata output and does not require external collaborators.
    e2e:
        required: false
        commands: []
        rationale: Unit coverage is sufficient for this repository-independent command surface.
    mutation:
        required: false
        commands: []
        rationale: The implementation is narrow command wiring rather than branch-heavy domain logic.
    manual_test:
        required: true
        commands: []
        rationale: Confirms both invocation forms work from the CLI without repository context.
---

## Summary

Add a small version-reporting command so Tessariq exposes both `tessariq version` and `tessariq --version` with identical single-line output.

## Acceptance Criteria

- `tessariq version` exists and prints `tessariq v<version>`.
- `tessariq --version` exists and prints the same line as `tessariq version`.
- The command works outside a git repository and without `.tessariq/` state.
- Root help includes the `version` subcommand.
- `version --help` exposes only command-local help and no unrelated operational flags.

## Test Expectations

- Add unit tests for root help listing `version`.
- Add unit tests for `tessariq version`, `tessariq --version`, and output equality.
- Add a unit test for `tessariq version --help` ensuring the command description is present and unrelated flags are absent.
- Manual-test both invocation forms outside repository-dependent command flows.

## TDD Plan

1. RED: add a failing test for `tessariq --version` output.
2. GREEN: wire root Cobra version output and add the `version` subcommand.
3. GREEN: add help coverage so the command appears in root help and stays command-local.

## Notes

- Keep the output intentionally minimal: `tessariq v<version>`.
- The implementation should use Tessariq-native wording only and must not reference unrelated projects in repo-tracked files.
- 2026-04-01T20:21:40Z: Implemented tessariq version and root --version; evidence: go vet ./..., go test ./..., go run ./cmd/tessariq --version, planning/artifacts/manual-test/TASK-055-version-command-and-root-version-flag/20260401T202035Z/report.md, planning/artifacts/verify/task/TASK-055-version-command-and-root-version-flag/20260401T202133Z/report.json, planning/artifacts/verify/spec/sweep/20260401T202059Z/report.json
