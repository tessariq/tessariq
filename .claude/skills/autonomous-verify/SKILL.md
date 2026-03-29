---
name: autonomous-verify
description: Run deterministic verification against Tessariq tracked-work acceptance criteria and spec coverage
argument-hint: "[profile]"
---

# autonomous-verify

Run deterministic verification against Tessariq tracked-work acceptance criteria and spec coverage.

## Required Flow

1. Run `go run ./cmd/tessariq-workflow validate-state`.
2. Choose `task`, `implemented`, or `spec` profile.
3. Run `go run ./cmd/tessariq-workflow verify --profile <profile> --disposition report --json`.
4. Confirm plan and report artifacts were written under `planning/artifacts/verify/`.
5. Review unresolved findings.
6. Run `go run ./cmd/tessariq-workflow followups --mode create --min-severity medium --json` when findings deserve backlog treatment.
7. Run `go run ./cmd/tessariq-workflow refresh-state`.

## Rules

- verification-only runs must not mutate product code unless explicitly requested
- keep mutation testing in mind for logic-heavy changes and CI uses a 70% threshold
- keep evidence paths in reports and notes
