# Manual Testing Workflow Design

## Context

Tessariq's verification pipeline currently covers automated testing (unit, integration, e2e, mutation) and spec-conformity checks, but has no step where the coding agent exercises the built artifact directly against acceptance criteria. Automated tests validate code paths in isolation; manual testing validates that the assembled CLI behaves as a user would expect. Adding this step catches behavioral gaps that automated tests miss: incorrect output formatting, missing evidence files, wrong exit codes, and subtle UX regressions.

This design adds an agent-driven manual testing phase to the tracked-work lifecycle. The agent creates a test plan derived from task acceptance criteria, executes each step in a sandboxed tmp directory, reasons about failures, and produces a structured report.

## Scope

- New skill: `autonomous-manual-test`
- Workflow documentation updates: `development-workflow.md`, `human-workflow.md`, `autonomous-contract.md`
- Skill update: `autonomous-task` gains a manual testing step
- CLI enforcement: `finish --status done` validates manual test artifacts exist
- Task frontmatter: new `manual_test` tier in the `verification:` block
- Artifact format: `plan.md` and `report.md` under `planning/artifacts/manual-test/`

## Non-goals

- GUI or interactive testing (this is CLI tooling)
- Performance benchmarking
- Replacing any existing automated test tier
- External service interaction during manual tests

## Skill: autonomous-manual-test

### Identity

- Name: `autonomous-manual-test`
- Mirrored to both `.agents/skills/autonomous-manual-test/SKILL.md` and `.claude/skills/autonomous-manual-test/SKILL.md`
- `disable-model-invocation: true`
- `argument-hint: "[task-id]"`

### Required Flow

1. Run `go run ./cmd/tessariq-workflow validate-state`.
2. Read the target task file and extract acceptance criteria.
3. Create a manual test plan at `planning/artifacts/manual-test/<task-id>/<timestamp>/plan.md`.
   - Each acceptance criterion becomes one or more numbered test steps.
   - Each step has: ID (MT-001, MT-002, ...), description, command(s) to run, expected outcome, and severity (critical, major, minor).
4. Execute each test step sequentially, using `/tmp/tessariq-manual-test-<task-id>/` as the working sandbox for all I/O.
5. On failure, apply severity-based resolution:
   - **Critical**: must fix the code and re-run the step. If the fix fails after one attempt, stop testing and report.
   - **Major**: attempt one fix and re-run. If the fix fails, log the failure and continue to the next step.
   - **Minor**: log the observation and continue to the next step.
6. Write `planning/artifacts/manual-test/<task-id>/<timestamp>/report.md` with per-step results and a summary verdict.
7. Clean up the `/tmp/tessariq-manual-test-<task-id>/` sandbox.

### Rules

- All file I/O during test execution must happen in `/tmp/tessariq-manual-test-<task-id>/`.
- The agent classifies each failure's severity and decides autonomously how to proceed.
- If a critical test cannot be fixed, the agent must not finish the task as `done`.
- The skill must not mutate test expectations to make them pass; fixes apply to product code only.
- Re-run only the specific failing step after a fix, not the entire plan.

## Artifact Format

### plan.md

```markdown
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

### MT-002: ...
```

### report.md

```markdown
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

### Verdict rules

- **pass**: all steps passed on first run.
- **pass-with-fixes**: all steps passed, but one or more required a code fix.
- **fail**: one or more critical or major steps failed and could not be fixed.

## Artifact Directory Structure

```
planning/artifacts/manual-test/
  <task-id>/
    <ISO-8601-timestamp>/
      plan.md
      report.md
```

Parallels the existing `planning/artifacts/verify/<profile>/<target>/<timestamp>/` convention.

## Workflow Integration

### autonomous-task skill

Insert manual testing as step 8, shifting subsequent steps:

```
1. Run validate-state.
2. Read the target task file.
3. Run start.
4. Implement in a TDD loop.
5. Follow the testing pyramid.
6. Use Testcontainers for integration/e2e.
7. Run mutation testing for non-trivial logic.
8. Run manual testing using the autonomous-manual-test skill.     <-- NEW
9. Run verify --profile task --disposition hybrid --json.
10. Create follow-up items for backlog-worthy findings.
11. Finish as blocked or done.
12. Run refresh-state.
```

### development-workflow.md

Add a "Manual Testing" section between "Mutation Testing" and "Tracked-Work Commands":

```markdown
## Manual Testing

After automated test tiers pass, run the manual testing skill to exercise the built CLI
against the task's acceptance criteria:

1. The agent reads the task's acceptance criteria and generates a test plan.
2. Each test step runs in a sandboxed `/tmp/` directory.
3. Failures are classified by severity (critical, major, minor) and resolved inline when possible.
4. A structured report records all outcomes.
5. Artifacts are written to `planning/artifacts/manual-test/<task-id>/<timestamp>/`.

Manual testing is required before running verification and before finishing a task as `done`.
```

### human-workflow.md

Add step 4.5:

```
4. Run the required test tiers according to the testing pyramid.
4.5. Run manual testing against acceptance criteria.                <-- NEW
5. Run verification for the task.
```

### autonomous-contract.md

Add to the "Verification Contract" section:

```markdown
- Manual test artifacts (plan and report) must exist under
  `planning/artifacts/manual-test/<task-id>/` before a task can be finished as `done`.
- `finish --status done` validates the presence of these artifacts.
```

## CLI Enforcement

### finish command gate

In `internal/workflow/service.go`, the `Finish` method gains a new check when `status == "done"`:

1. Resolve the artifact path: `planning/artifacts/manual-test/<task-id>/`.
2. Verify at least one timestamped subdirectory exists.
3. In the most recent subdirectory, verify both `plan.md` and `report.md` exist.
4. If any check fails, return an error: `"manual test artifacts missing for task <task-id>; run manual testing before finishing as done"`.

This check is skipped for `blocked` and `cancelled` statuses, since a task may be blocked precisely because manual testing found unfixable issues.

### validate-state awareness

`validate-state` does not enforce manual test artifacts (tasks in `todo` or `in_progress` won't have them yet). The gate is exclusively in `finish`.

## Task Frontmatter Extension

Add a `manual_test` tier to the `verification:` block in task files:

```yaml
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: ...
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: ...
    e2e:
        required: false
        commands:
            - go test -tags=e2e ./...
        rationale: ...
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: ...
    manual_test:
        required: true
        rationale: Validates CLI output and evidence artifacts through direct execution against acceptance criteria.
```

The `manual_test` tier has no `commands` field because the test plan is generated dynamically from acceptance criteria, not from a fixed command.

## Change-Type Matrix Update

Add to `development-workflow.md` Change-Type Matrix:

```
- All code changes:
  manual testing against task acceptance criteria before verification
```

## Files to modify

1. **New**: `.agents/skills/autonomous-manual-test/SKILL.md` and `.claude/skills/autonomous-manual-test/SKILL.md` (mirrored)
2. **Edit**: `.agents/skills/autonomous-task/SKILL.md` and `.claude/skills/autonomous-task/SKILL.md` (add step 8)
3. **Edit**: `docs/workflow/development-workflow.md` (add Manual Testing section, update Change-Type Matrix)
4. **Edit**: `docs/workflow/human-workflow.md` (add step 4.5)
5. **Edit**: `docs/workflow/autonomous-contract.md` (add manual test artifact requirement)
6. **Edit**: `internal/workflow/service.go` (add manual test artifact check in Finish)
7. **Edit**: `internal/workflow/service_test.go` (test the new finish gate)
8. **Edit**: `AGENTS.md` (add manual testing to the change checklist)
9. **Edit**: All `planning/tasks/TASK-*.md` files (add `manual_test` tier to verification frontmatter)

## Verification

To verify this change end-to-end:

1. Build: `go build ./cmd/tessariq-workflow`
2. Unit tests: `go test ./internal/workflow/...` — the new finish gate test should pass
3. Full test suite: `go test ./...`
4. Skill parity: `go run ./cmd/tessariq-workflow check-skills` — mirrored skills must match
5. State validation: `go run ./cmd/tessariq-workflow validate-state`
6. Manual check: attempt `finish --status done` on a task without manual test artifacts — should fail
7. Manual check: create mock artifacts, retry finish — should succeed
