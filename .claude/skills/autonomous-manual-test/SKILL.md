---
name: autonomous-manual-test
description: Generate and execute a manual test plan against task acceptance criteria in a sandboxed tmp directory
disable-model-invocation: true
argument-hint: "[task-id]"
---

# autonomous-manual-test

Generate and execute a manual test plan against task acceptance criteria, then produce a structured report.

## Required Flow

1. Run `go run ./cmd/tessariq-workflow validate-state`.
2. Read the target task file and extract acceptance criteria.
3. Create a manual test plan at `planning/artifacts/manual-test/<task-id>/<timestamp>/plan.md`.
   - Each acceptance criterion becomes one or more numbered test steps (MT-001, MT-002, ...).
   - Each step has: ID, description, command(s) to run, expected outcome, and severity (critical, major, minor).
4. Execute each test step sequentially, using `/tmp/tessariq-manual-test-<task-id>/` as the working sandbox for all I/O.
5. On failure, apply severity-based resolution:
   - **Critical**: fix the code and re-run the step. If the fix fails after one attempt, stop testing and write the report.
   - **Major**: attempt one fix and re-run. If the fix fails, log the failure and continue to the next step.
   - **Minor**: log the observation and continue to the next step.
6. Write `planning/artifacts/manual-test/<task-id>/<timestamp>/report.md` with per-step results and a summary verdict.
7. Clean up the `/tmp/tessariq-manual-test-<task-id>/` sandbox.

## Artifact Format

### plan.md

```
# Manual Test Plan

- Task: <task-id>
- Generated: <ISO-8601 timestamp>
- Sandbox: /tmp/tessariq-manual-test-<task-id>/

## Test Steps

### MT-001: <description derived from acceptance criterion>

- Severity: critical | major | minor
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

- All file I/O during test execution must happen in `/tmp/tessariq-manual-test-<task-id>/`.
- The agent classifies each failure's severity and decides autonomously how to proceed.
- If a critical test cannot be fixed, the agent must not finish the task as `done`.
- Fixes apply to product code only; never mutate test expectations to force a pass.
- Re-run only the specific failing step after a fix, not the entire plan.
- Do not auto-select another task after manual testing completes.
