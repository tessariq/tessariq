---
id: T-123-rename-specs-to-taskrail-layout
title: Rename versioned specs to taskrail's specs/<version>.md layout
status: todo
priority: high
spec_ref: specs/tessariq-v0.2.0.md#release-intent
dependencies: []
updated_at: "2026-08-01T08:26:50Z"
---

# T-123-rename-specs-to-taskrail-layout Rename versioned specs to taskrail's specs/<version>.md layout

## Description

The `taskrail spec` command family hardcodes the spec path `specs/<version>.md`.
This repository names its specs `specs/tessariq-<version>.md`, so taskrail never
discovers them as versioned specs. `.taskrail/config.yml` sets `specs_dir` but
offers no filename-pattern knob, so the layout must change on our side.

`taskrail validate` passes regardless, because it resolves `spec_ref` values as
literal paths. Only the spec-registry commands break. Observed against the
current tree (reproduced in a scratch copy so no state was written):

| Command | Result today |
|---|---|
| `taskrail validate` | `state valid` |
| `taskrail spec list` | `no versioned specs found` |
| `taskrail spec show v0.1.0 --anchors` | `read spec file specs/v0.1.0.md: no such file or directory` |
| `taskrail spec diff v0.1.0 v0.2.0` | same error |
| `taskrail spec activate v0.2.0` | `spec file specs/v0.2.0.md does not exist` |
| `taskrail task new --area <anchor>` | unusable; `--spec-ref` is the only working form |

`spec activate` is the blocking one: it is the supported way to repoint
`STATE.md` at v0.2.0 once v0.1.0 ships, and it cannot run today. Renaming the
files restores the whole family — verified in a scratch copy, where after the
rename `taskrail spec list` reported all three specs with `v0.1.0 (active)` and
`taskrail validate` still returned `state valid`.

This unblocks [T-124-define-v020-coverable-areas](T-124-define-v020-coverable-areas.md),
which needs `spec show --anchors` and `coverage --area` to work.

Scope note: this is a file-layout and reference rewrite only. No spec prose
changes, so the normative content is untouched.

## Acceptance

- `specs/tessariq-v0.1.0.md`, `specs/tessariq-v0.2.0.md`, and
  `specs/tessariq-v0.3.0.md` are renamed to `specs/v0.1.0.md`, `specs/v0.2.0.md`,
  and `specs/v0.3.0.md` via `git mv`, so history follows the rename.
- Spec prose is byte-identical across the rename; `git log --follow` resolves and
  the only diff is the path.
- Every `spec_ref` in `planning/tasks/*.md` is rewritten to the new path. All 124
  task files keep a resolvable `spec_ref`.
- `planning/STATE.md` `active_spec_path` is updated. It must be rewritten by
  taskrail, not by hand — use `taskrail spec activate v0.1.0` (re-activating the
  already-active spec) or `taskrail repair --apply`, and never hand-edit
  frontmatter.
- Prose references to the old filenames are updated in: `README.md`, `AGENTS.md`,
  `specs/README.md`, `planning/README.md`, `planning/BUGS.md`,
  and `docs/release-verification.md`.
  130 files reference the old names in total; `.github/workflows/*.yml` use
  `specs/**` globs and need no change.
- `taskrail validate` returns `state valid`.
- `taskrail spec list` lists all three specs and marks `v0.1.0` active.
- `taskrail spec show v0.1.0 --anchors` and `taskrail spec diff v0.1.0 v0.2.0`
  both succeed.
- `task workflow:validate` and `task workflow:verify:spec` pass.
- The CI planning lane passes; `internal/toolchain` path-list parity is unaffected
  but must still be green.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
