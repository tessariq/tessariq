---
name: autonomous-backlog
description: Execute one deterministic autonomous backlog cycle for Tessariq tracked work
---

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
9. If the task involves Docker containers or bind mounts, verify: mount modes (ro vs rw per mount), container user is non-root and non-host, UID alignment for writable bind mounts (chmod before start), and cleanup permissions (chmod before removal). See AGENTS.md "Container user and bind-mount permissions".
10. Run mutation testing for non-trivial logic changes and keep the 70% threshold in mind.
11. Run `go run ./cmd/tessariq-workflow verify --profile task --task <task-id> --disposition hybrid --json`.
12. If unresolved medium-or-higher findings remain, run `go run ./cmd/tessariq-workflow followups --mode create --min-severity medium --json`.
13. Finish as `blocked` when unresolved high-severity findings remain; otherwise finish as `done`.
14. Run `go run ./cmd/tessariq-workflow refresh-state`.

## Rules

- never hand-edit `planning/STATE.md` frontmatter
- never hand-edit task status fields
- always keep evidence paths in finish notes
- produce exactly one conventional-commit commit per implemented task, including code/test and required workflow/planning updates together
- do not create a second chore/workflow-only commit for the same task
- stop after one item
