# Human Workflow

How a human developer should work when Tessariq tracked-work state exists.

## Normal Flow

1. Run `go run ./cmd/tessariq-workflow validate-state`.
2. Claim a tracked item with `start`.
3. Implement in a TDD loop.
4. Run the required test tiers according to the testing pyramid.
5. Run manual testing against the task's acceptance criteria.
6. Run verification for the task.
7. Create follow-up items if verification leaves backlog-worthy findings.
8. Finish the task with an evidence-bearing note.
9. Refresh state.

## Testing Rules

- Unit tests stay in-memory only.
- Integration and e2e tests may use temp files and temp workspaces, but collaborators must come from Testcontainers for Go.
- Do not spin up custom HTTP or TCP servers in integration or e2e tests.
- Mutation testing is required for non-trivial logic changes and CI enforces a 70% threshold.
- `planning/artifacts/` is local-only workflow evidence and is gitignored.
- If `followups` cannot find a local verification report for the recorded validation run, rerun `verify` before creating follow-up tasks.

## Recovery

If state is stale or inconsistent:

1. Run `validate-state --json`.
2. Run `next --json` once to apply deterministic recovery.
3. Run `validate-state --json` again.
4. If state is still invalid, stop and fix the workflow tooling instead of editing files by hand.
