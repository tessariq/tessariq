---
id: TASK-055-version-command-and-root-version-flag
title: Add version command and root version flag
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#tessariq-version
dependencies: []
updated_at: "2026-04-01T20:21:40Z"
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
- 2026-04-01T20:21:40Z: Implemented tessariq version and root --version; evidence: go vet ./..., go test ./..., go run ./cmd/tessariq --version, (evidence artifacts; path omitted) (evidence artifacts; path omitted) (evidence artifacts; path omitted)
