---
name: autonomous-verify-hybrid
description: Run hybrid verification for Tessariq tracked work with deterministic low-risk fixes only
argument-hint: "[profile]"
---

# autonomous-verify-hybrid

Run hybrid verification for Tessariq tracked work with deterministic low-risk fixes only.

## Required Flow

1. Run `go run ./cmd/tessariq-workflow validate-state`.
2. Run `go run ./cmd/tessariq-workflow verify --profile <profile> --disposition hybrid --json`.
3. Confirm plan and report artifacts were written under `planning/artifacts/verify/`.
4. Run mutation testing when logic-confidence evidence is otherwise weak; CI enforces a 70% threshold.
5. If unresolved findings remain, run `go run ./cmd/tessariq-workflow followups --mode create --min-severity medium --json`.
6. If an active task exists, finish as `blocked` when unresolved high-severity findings remain; otherwise finish as `done`.
7. Run `go run ./cmd/tessariq-workflow refresh-state`.

## Rules

- auto-fix only deterministic low-risk issues such as regenerated machine-owned outputs
- never hand-edit machine-managed state
- never hand-edit task status
