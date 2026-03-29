# Autonomous Contract

Deterministic tracked-work contract for Tessariq repository planning and implementation.

## Source Of Truth

- `planning/STATE.md` frontmatter is machine-managed run state.
- `planning/tasks/` contains tracked work item metadata and acceptance criteria.
- `docs/workflow/` contains the human-readable workflow contract.
- `.agents/skills/` and `.claude/skills/` contain mirrored agent skills and must stay byte-identical.
- `go run ./cmd/tessariq-workflow ...` is the only valid transition path for tracked-work state.

## Lifecycle

Recommended task status lifecycle:

- `todo`
- `in_progress`
- terminal: `done`, `blocked`, `cancelled`

Rules:

- At most one tracked item may be `in_progress`.
- `planning/STATE.md` must point at the same active task.
- A stale active task is invalid until deterministic recovery runs through `next`.
- Human or agent users must not hand-edit machine-managed state or status transitions.

## Deterministic Selection

`go run ./cmd/tessariq-workflow next --json` selects work in this order:

1. Continue the non-stale active task, if present.
2. Otherwise recover a stale task deterministically.
3. Choose eligible `todo` items with resolved dependencies.
4. Sort by priority, then stable task identifier.
5. Persist the ordered candidate list in `planning/STATE.md`.

## Verification Contract

- Run verification through `go run ./cmd/tessariq-workflow verify ...`.
- `task` profile checks one tracked item.
- `implemented` profile checks completed items for retained verification metadata.
- `spec` profile checks seeded task coverage against `specs/tessariq-v0.1.0.md`.
- Verification writes plan and report artifacts under `planning/artifacts/verify/`.
- Unresolved medium-or-higher findings can be converted into tracked follow-up items through `followups --mode create`.
- Manual test artifacts (plan and report) must exist under `planning/artifacts/manual-test/<task-id>/` before a task can be finished as `done`.
- `finish --status done` validates the presence of these artifacts.

## Safety Rules

- Never hand-edit machine-managed state.
- Never hand-edit task status fields.
- Keep workflow commands non-interactive and scriptable.
- Completion notes must point to concrete evidence.
- Code-changing work must follow TDD and repository testing rules.
