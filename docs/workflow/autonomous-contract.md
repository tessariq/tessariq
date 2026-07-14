# Autonomous Contract

Deterministic tracked-work contract for Tessariq repository planning and implementation.

## Source Of Truth

- `planning/STATE.md` frontmatter is machine-managed run state.
- `planning/tasks/` contains tracked work item metadata and acceptance criteria.
- `docs/workflow/` contains the human-readable workflow contract.
- `.agents/skills/` and `.claude/skills/` contain mirrored agent skills and must stay byte-identical.
- `taskrail <cmd>` is the only valid transition path for tracked-work state.

## Lifecycle

Recommended task status lifecycle:

- `todo`
- `active`
- terminal: `completed`, `blocked`

Rules:

- At most one tracked item may be `active`.
- `planning/STATE.md` must point at the same active task.
- A stale active task is invalid until deterministic recovery runs through `taskrail repair`.
- Human or agent users must not hand-edit machine-managed state or status transitions.

## Deterministic Selection

`taskrail next --json` selects work in this order:

1. Continue the non-stale active task, if present.
2. Otherwise recover a stale task deterministically.
3. Choose eligible `todo` items with resolved dependencies.
4. Sort by priority, then stable task identifier.
5. Persist the ordered candidate list in `planning/STATE.md`.

## Verification Contract

- Run task verification through `taskrail verify <id> --result pass|fail --summary "<s>" [--details "<d>"]`.
- `taskrail verify <id>` checks one tracked item and records its verification result.
- Run advisory spec coverage through `taskrail coverage --json`; it is read-only and reports seeded task coverage against `specs/tessariq-v0.1.0.md`.
- Unresolved findings can be converted into tracked follow-up items through `taskrail verify <id> --create-followup --followup-title "<t>" --followup-description "<d>" [--followup-priority high|medium|low]`.
- Manual test artifacts (plan and report) must exist locally under `planning/artifacts/manual-test/<task-id>/` before a task can be finished as `completed`.
- `taskrail complete` validates the presence of these artifacts.

## Safety Rules

- Never hand-edit machine-managed state.
- Never hand-edit task status fields.
- Keep workflow commands non-interactive and scriptable.
- Completion notes must point to concrete evidence.
- Code-changing work must follow TDD and repository testing rules.
- For implementation tasks, commit all task-related changes (product code, tests, and required planning/workflow metadata updates) in a single conventional-commit commit; never commit files under `planning/artifacts/`.
- Do not create a second commit only for task/workflow/verification metadata updates.
