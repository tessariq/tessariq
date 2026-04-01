---
name: autonomous-manual-test
description: Generate and execute a manual test plan against task acceptance criteria in a sandboxed tmp directory
argument-hint: "[task-id]"
---

# autonomous-manual-test

Generate and execute a manual test plan against task acceptance criteria, then produce a structured report.

## Required Flow

1. Run `go run ./cmd/tessariq-workflow validate-state`.
2. Read the target task file and extract acceptance criteria.
3. Create a local-only manual test plan at `planning/artifacts/manual-test/<task-id>/<timestamp>/plan.md`.
   - Each acceptance criterion becomes one or more numbered test steps (MT-001, MT-002, ...).
   - Each step has: ID, description, command(s) to run, expected outcome, and severity (critical, major, minor).
4. Choose the right test mode for each step (see Test Modes below).
5. Execute each test step sequentially.
6. On failure, apply severity-based resolution:
   - **Critical**: fix the code and re-run the step. If the fix fails after one attempt, stop testing and write the report.
   - **Major**: attempt one fix and re-run. If the fix fails, log the failure and continue to the next step.
   - **Minor**: log the observation and continue to the next step.
7. Write the local-only `planning/artifacts/manual-test/<task-id>/<timestamp>/report.md` with per-step results and a summary verdict.
8. Clean up any sandbox directories created during testing.

## Test Modes

### Sandbox mode (default)

For tests that exercise Go APIs, unit logic, or simple CLI commands without runtime dependencies like tmux or Docker containers.

- Use `/tmp/tessariq-manual-test-<task-id>/` as the working sandbox for all I/O.
- Write standalone Go programs that import internal packages and run via `go run`.
- Place test programs inside the module (e.g. `cmd/manual-test-NNN/main.go`) to access internal packages.
- Clean up `cmd/manual-test-NNN/` and the sandbox after testing.

### Container mode

For tests that need process collaborators, runtime dependencies (tmux, git, docker), or full CLI lifecycle execution. This mode is **required** when:
- The test needs tmux (e.g. `tessariq run` creates tmux sessions)
- The test needs a fake adapter binary (e.g. fake `claude`)
- The test exercises the full CLI inside an isolated environment

Container mode rules:
- Write tests as `_manual_test.go` files with build tag `//go:build manual_test`.
- Place the test file in the package closest to the code under test (e.g. `internal/adapter/claudecode/claudecode_manual_test.go`).
- Use `_test` package suffix (external test package) so imports are explicit.
- Use Testcontainers helpers from `internal/testutil/containers/` (`StartRunEnv`, `StartAgentEnv`, etc.).
- Run via `go test -tags=manual_test ./<package>/ -run TestManual_<Name> -v -count=1`.
- Name test functions `TestManual_<descriptive name>` for easy grep and filtering.
- Build CLI binaries with `CGO_ENABLED=0` when they need to run inside Alpine containers.
- Never use `skipIfNoTmux` or similar host-tool guards — the container provides everything.

### Choosing the right mode

| Scenario | Mode |
|----------|------|
| Test calls Go functions directly | Sandbox |
| Test runs `go test` on existing tests | Sandbox |
| Test runs CLI that needs tmux | Container |
| Test needs fake adapter binary | Container |
| Test verifies evidence artifacts from a real run | Container |
| Test exercises API or struct behavior | Sandbox |

## Artifact Format

### plan.md

```
# Manual Test Plan

- Task: <task-id>
- Generated: <ISO-8601 timestamp>
- Sandbox: /tmp/tessariq-manual-test-<task-id>/ (sandbox mode)
  OR: Testcontainers RunEnv (container mode)

## Test Steps

### MT-001: <description derived from acceptance criterion>

- Severity: critical | major | minor
- Mode: sandbox | container
- Derived from: <quoted or paraphrased acceptance criterion>
- Setup: <any preconditions or fixture creation>
- Command: `<shell command to execute>`
- Expected: <observable outcome: exit code, file existence, output content, etc.>
```

### report.md

```
# Manual Test Report

- Task: <task-id>
- Executed: <ISO-8601 timestamp>
- Verdict: pass | pass-with-fixes | fail

## Results

### MT-001: <description>

- Status: pass | fail | fixed | skipped
- Observation: <what actually happened>
- Fix: <if status is "fixed", describe the code change with file:line>
- Re-run: <pass | fail, only if a fix was applied>

## Summary

- Total: N | Pass: N | Fixed: N | Failed: N | Skipped: N
```

### Verdict Rules

- **pass**: all steps passed on first run.
- **pass-with-fixes**: all steps passed, but one or more required a code fix.
- **fail**: one or more critical or major steps failed and could not be fixed.

## Rules

- The agent classifies each failure's severity and decides autonomously how to proceed.
- If a critical test cannot be fixed, the agent must not finish the task as `done`.
- Fixes apply to product code only; never mutate test expectations to force a pass.
- Re-run only the specific failing step after a fix, not the entire plan.
- Do not auto-select another task after manual testing completes.
- Never take shortcuts by substituting automated e2e test results for manual test evidence. If a test needs containers, write a proper `_manual_test.go` file.

### Cleanup (critical)

Manual test code is **ephemeral tooling**. The `plan.md` and `report.md` artifacts remain only as local gitignored evidence under `planning/artifacts/`. Test code must never be committed.

- Sandbox mode: delete `cmd/manual-test-NNN/` and `/tmp/tessariq-manual-test-<task-id>/` after the report is written.
- Container mode: delete `_manual_test.go` files after the report is written.
- `.gitignore` blocks `*_manual_test.go`, `cmd/manual-test-*/`, and `planning/artifacts/` as a safety net, but the agent must still clean up explicitly.
- If manual test code is found in a commit, remove it immediately.
- Never commit files under `planning/artifacts/`.
