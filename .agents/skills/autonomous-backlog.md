# autonomous-backlog

Execute one deterministic autonomous backlog cycle for Tessariq tracked work.

## Required Flow

1. Run `go run ./cmd/tessariq-workflow validate-state`.
2. Run `go run ./cmd/tessariq-workflow next --json`.
3. If no task is eligible, report that and stop.
4. Read the selected task file under `planning/tasks/`.
5. Run `go run ./cmd/tessariq-workflow start --mode autonomous_backlog --agent-id <runtime> --model <model> <task-id>`.
6. Implement in a TDD loop.
7. Prefer unit tests first, then integration tests, then e2e tests, following the testing pyramid.
8. Use Testcontainers for Go for any integration or e2e collaborator; do not create custom local servers.
9. Run mutation testing for non-trivial logic changes and keep the 70% threshold in mind.
10. Run `go run ./cmd/tessariq-workflow verify --profile task --task <task-id> --disposition hybrid --json`.
11. If unresolved medium-or-higher findings remain, run `go run ./cmd/tessariq-workflow followups --mode create --min-severity medium --json`.
12. Finish as `blocked` when unresolved high-severity findings remain; otherwise finish as `done`.
13. Run `go run ./cmd/tessariq-workflow refresh-state`.

## Rules

- never hand-edit `planning/STATE.md` frontmatter
- never hand-edit task status fields
- always keep evidence paths in finish notes
- stop after one item
