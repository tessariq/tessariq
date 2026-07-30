# Planning Scope

`planning/tasks/` is usually milestone-scoped tracked work, but it is not restricted to milestone delivery only.

Current default scope:

- active milestone: `v0.1.0`
- active spec: `specs/tessariq-v0.1.0.md`

Exceptions:

- bug fixes may be tracked outside the current milestone when they need immediate repair
- small nice-to-have tasks may be tracked when they are intentionally accepted outside the current milestone theme

Rules:

- `planning/STATE.md` declares the current default milestone/spec scope
- milestone-scoped tasks must match that declared scope
- exception tasks must still declare a correct `spec_ref` (and may retain the legacy `spec_refs` list) and should only diverge from the default scope intentionally
- every task `spec_ref` must point to a live heading in the referenced spec
- `taskrail validate` is the hard structural gate for task metadata and spec links
- `taskrail coverage --json` reports advisory coverage for the active milestone spec

## Task ids

Task ids follow Taskrail's native scheme: `T-<zero-padded-number>`, stored one
per file at `planning/tasks/T-<number>.md`. The descriptive title lives in the
`title` frontmatter field, not in the filename.

Create tasks with `taskrail task new`, which allocates the next free id in this
scheme. Do not hand-author task files or invent a different id format: the
binary derives the next id from the existing corpus, so an unrecognized format
makes it restart numbering and collide with existing work.

When the default milestone changes, update `planning/STATE.md`, reseed milestone tasks as needed, and regenerate verification artifacts.
