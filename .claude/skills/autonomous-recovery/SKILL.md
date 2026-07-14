---
name: autonomous-recovery
description: Recover Tessariq tracked-work state when it becomes stale or inconsistent
---

# autonomous-recovery

Recover Tessariq tracked-work state when it becomes stale or inconsistent.

## Required Flow

1. Run `taskrail validate`.
2. If stale or inconsistent state exists, run `taskrail repair --apply` once to apply deterministic recovery.
3. Run `taskrail validate` again.
4. Report exact violations if state remains invalid.

## Rules

- never implement product code during recovery-only runs unless explicitly requested
- never hand-edit machine-managed state
- never hand-edit task status
