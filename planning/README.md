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
- exception tasks must still declare a correct `spec_ref` and should only diverge from the default scope intentionally
- every task `spec_ref` must point to a live heading in the referenced spec

Use Taskrail's spec workflow when the default milestone changes; never hand-edit managed `planning/STATE.md` state.
