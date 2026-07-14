---
name: autonomous-verify-hybrid
description: Run hybrid verification for Tessariq tracked work with deterministic low-risk fixes only
argument-hint: "[profile]"
---

# autonomous-verify-hybrid

Run hybrid verification for Tessariq tracked work with deterministic low-risk fixes only.

## Required Flow

1. Run `taskrail validate`.
2. Run `taskrail verify <task-id> --result pass|fail --summary "<s>" [--details "<d>"]` for the task under review, or `taskrail coverage --json` for advisory spec coverage.
3. Apply only deterministic low-risk fixes (see Rules) and re-run the verification.
4. Run mutation testing when logic-confidence evidence is otherwise weak; CI enforces a 70% threshold.
5. If unresolved findings remain, create a follow-up with `taskrail verify <task-id> --create-followup --followup-title "<t>" --followup-description "<d>" [--followup-priority high|medium|low]`.
6. If an active task exists, finish with `taskrail block --reason "<n>" <task-id>` when unresolved high-severity findings remain; otherwise finish with `taskrail complete --note "<n>" <task-id>`.
7. Run `taskrail repair --apply`.

## Rules

- auto-fix only deterministic low-risk issues such as regenerated machine-owned outputs
- never hand-edit machine-managed state
- never hand-edit task status
- when code changes are made for an active task, keep them in a single conventional-commit commit that also includes required workflow/planning metadata updates
