# autonomous-recovery

Recover Tessariq tracked-work state when it becomes stale or inconsistent.

## Required Flow

1. Run `go run ./cmd/tessariq-workflow validate-state --json`.
2. If stale or inconsistent state exists, run `go run ./cmd/tessariq-workflow next --json` once to apply deterministic recovery.
3. Run `go run ./cmd/tessariq-workflow validate-state --json` again.
4. If state is valid, run `go run ./cmd/tessariq-workflow refresh-state`.
5. Report exact violations if state remains invalid.

## Rules

- never implement product code during recovery-only runs unless explicitly requested
- never hand-edit machine-managed state
- never hand-edit task status
