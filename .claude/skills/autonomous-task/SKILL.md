---
name: autonomous-task
description: Execute one specified Tessariq tracked task with deterministic workflow transitions
argument-hint: "[task-id]"
---

# autonomous-task

Execute one specified Tessariq tracked task with deterministic workflow transitions.

## Required Flow

1. Run `go run ./cmd/tessariq-workflow validate-state`.
2. Read the target task file.
3. Run `go run ./cmd/tessariq-workflow start --mode user_request --agent-id <runtime> --model <model> <task-id>`.
4. Implement only the requested scope in a TDD loop.
5. Follow the testing pyramid and keep unit tests dominant.
6. Use Testcontainers for Go for integration and e2e collaborators; do not create custom local servers.
7. Run mutation testing for non-trivial logic changes with the 70% threshold in mind.
8. Run manual testing using the `autonomous-manual-test` skill against the task's acceptance criteria.
9. Run `go run ./cmd/tessariq-workflow verify --profile task --task <task-id> --disposition hybrid --json`.
10. When unresolved medium-or-higher findings deserve backlog treatment, run `go run ./cmd/tessariq-workflow followups --mode create --min-severity medium --json`.
11. Finish as `blocked` when unresolved high-severity findings remain; otherwise finish as `done`.
12. Run `go run ./cmd/tessariq-workflow refresh-state`.

## Rules

- do not auto-select another task
- do not hand-edit machine-managed state
- do not hand-edit task status
- produce exactly one conventional-commit commit for the task, including code/test and required workflow/planning updates together
- do not split the task into separate implementation and chore/workflow-update commits
