---
name: autonomous-task
description: Execute one specified Tessariq tracked task with deterministic workflow transitions
argument-hint: "[task-id]"
---

# autonomous-task

Execute one specified Tessariq tracked task with deterministic workflow transitions.

## Required Flow

1. Run `taskrail validate`.
2. Read the target task file.
3. Run `taskrail start <task-id>`.
4. Implement only the requested scope in a TDD loop.
5. Follow the testing pyramid and keep unit tests dominant.
6. Use Testcontainers for Go for integration and e2e collaborators; do not create custom local servers.
7. Run mutation testing for non-trivial logic changes with the 70% threshold in mind.
8. Run manual testing using the `autonomous-manual-test` skill against the task's acceptance criteria.
9. Run `taskrail verify <task-id> --result pass|fail --summary "<s>" [--details "<d>"]`.
10. When unresolved medium-or-higher findings deserve backlog treatment, create a follow-up with `taskrail verify <task-id> --create-followup --followup-title "<t>" --followup-description "<d>" [--followup-priority high|medium|low]`.
11. Finish with `taskrail block --reason "<n>" <task-id>` when unresolved high-severity findings remain; otherwise finish with `taskrail complete --note "<n>" <task-id>`.
12. Run `taskrail repair --apply`.

## Rules

- do not auto-select another task
- do not hand-edit machine-managed state
- do not hand-edit task status
- produce exactly one conventional-commit commit for the task, including code/test and required workflow/planning updates together
- do not split the task into separate implementation and chore/workflow-update commits
