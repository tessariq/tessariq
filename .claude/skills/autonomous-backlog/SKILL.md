---
name: autonomous-backlog
description: Execute one deterministic autonomous backlog cycle for Tessariq tracked work
---

# autonomous-backlog

Execute one deterministic autonomous backlog cycle for Tessariq tracked work.

## Required Flow

1. Run `taskrail validate`.
2. Run `taskrail next --json`.
3. If no task is eligible, report that and stop.
4. Read the selected task file under `planning/tasks/`.
5. Run `taskrail start <task-id>`.
6. Implement in a TDD loop.
7. Prefer unit tests first, then integration tests, then e2e tests, following the testing pyramid.
8. Use Testcontainers for Go for any integration or e2e collaborator; do not create custom local servers.
9. If the task involves Docker containers or bind mounts, verify: mount modes (ro vs rw per mount), container user is non-root and non-host, UID alignment for writable bind mounts (chmod before start), and cleanup permissions (chmod before removal). See AGENTS.md "Container user and bind-mount permissions".
10. Run mutation testing for non-trivial logic changes and keep the 70% threshold in mind.
11. Run `taskrail verify <task-id> --result pass|fail --summary "<s>" [--details "<d>"]`.
12. If unresolved medium-or-higher findings deserve backlog treatment, create a follow-up with `taskrail verify <task-id> --create-followup --followup-title "<t>" --followup-description "<d>" [--followup-priority high|medium|low]`.
13. Finish with `taskrail block --reason "<n>" <task-id>` when unresolved high-severity findings remain; otherwise finish with `taskrail complete --note "<n>" <task-id>`.
14. Run `taskrail repair --apply`.

## Rules

- never hand-edit `planning/STATE.md` frontmatter
- never hand-edit task status fields
- always keep evidence paths in finish notes
- produce exactly one conventional-commit commit per implemented task, including code/test and required workflow/planning updates together
- do not create a second chore/workflow-only commit for the same task
- stop after one item
