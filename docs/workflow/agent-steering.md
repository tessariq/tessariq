# Agent Steering

Prompt guidance for deterministic tracked-work execution in Tessariq.

## Baseline Rules

- Use `go run ./cmd/tessariq-workflow ...` for all tracked-work transitions.
- Do not hand-edit `planning/STATE.md` frontmatter.
- Do not hand-edit task status fields in `planning/tasks/`.
- Follow TDD for code changes.
- Follow the testing pyramid and repository testing rules.
- Use Testcontainers for Go for integration and e2e collaborators; do not create custom local servers.
- Treat mutation testing with a 70% threshold as part of normal CI-quality validation.

## Autonomous Backlog

1. Validate state.
2. Select the next eligible task deterministically.
3. Start the selected task.
4. Implement it in a TDD loop.
5. Run the appropriate test tiers.
6. Run task-scoped verification.
7. Create follow-up items for unresolved findings when needed.
8. Finish as `blocked` if unresolved high-severity findings remain; otherwise `done`.
9. Refresh state and report evidence.

## Directed Task

1. Validate state.
2. Read the requested task file.
3. Start that task only.
4. Implement only the requested scope.
5. Verify it and finish it through the workflow CLI.

## Verification Runs

Use verification-only runs when:

- you need a spec sweep
- you need to audit completed tasks
- you need to create follow-up tasks from unresolved findings

Verification-only runs must not mutate product code unless the task explicitly allows deterministic low-risk fixes.
