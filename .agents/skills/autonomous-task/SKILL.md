---
name: autonomous-task
description: Execute one specified Tessariq tracked task with deterministic workflow transitions
disable-model-invocation: true
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
8. Run `go run ./cmd/tessariq-workflow verify --profile task --task <task-id> --disposition hybrid --json`.
9. Create follow-up items when unresolved findings deserve backlog treatment.
10. Finish as `blocked` when unresolved high-severity findings remain; otherwise finish as `done`.
11. Run `go run ./cmd/tessariq-workflow refresh-state`.

## Rules

- do not auto-select another task
- do not hand-edit machine-managed state
- do not hand-edit task status
