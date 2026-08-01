---
id: T-124-define-v020-coverable-areas
title: Define coverable feature areas in the v0.2.0 spec
status: todo
priority: medium
spec_ref: specs/tessariq-v0.2.0.md#scope
dependencies:
    - T-123-rename-specs-to-taskrail-layout
updated_at: "2026-08-01T08:26:55Z"
---

# T-124-define-v020-coverable-areas Define coverable feature areas in the v0.2.0 spec

## Description

`taskrail coverage` reports nothing for this repository:

```
"coverage_percent": null,  "coverable_areas": 0,
"covered_areas": 0,        "areas": []
```

The cause is not missing task linkage — 115 of 124 tasks already anchor to real
spec headings (`#tessariq-run-task-path` x20, `#networking-and-egress` x19,
`#evidence-contract` x12, and 30+ distinct anchors). The cause is that taskrail
counts only `###` headings nested under a `## Potential Features` `##` heading as
coverable areas. Its own scaffold (`taskrail spec add`) states this:

> `_TODO: add `### Feature Area` headings here to define coverable areas.
> Until an area is added, this spec has zero coverable areas and `taskrail coverage`
> reports N/A for it._`

No Tessariq spec has that section (`grep -c "Potential Features"` returns 0 for
all three). Confirmed by experiment in a scratch copy: appending a
`## Potential Features` block with two `###` headings to v0.1.0 moved coverage
from `coverable_areas: 0` / `coverage_percent: null` to `coverable_areas: 2` /
`coverage_percent: 0`, with both areas listed and `covered: false`. Nothing else
changed.

Consequences while this stands:

- `taskrail coverage` and `coverage --min` are inert.
- The **taskrail-decompose** skill is a silent no-op — it selects `areas[]`
  entries with `"covered": false`, and `areas` is always empty.
- The **taskrail-gap** skill (`coverage --gaps`) analyses "covered areas", so it
  has nothing to analyse.

Only v0.2.0 is in scope here. v0.1.0's spec is a shipped normative contract and
should not be restructured on the release path; v0.3.0 is still a draft.

Per `AGENTS.md`, specs are normative and change only when explicitly requested.
This change was explicitly requested.

### Open decision for the implementer

Adding the section overlaps the existing `## Scope` list, and the six open
v0.2.0 tasks anchor to headings that will still sit outside `## Potential
Features` (`#release-intent`, `#shared-runtime-sketch`, `#evidence-additions`,
`#runner-responsibilities`). Two ways to resolve it:

- **A (recommended).** Add `## Potential Features` as an additive decomposition
  index; its `###` areas name capabilities and cross-link the normative sections
  that define them. Accepts prose duplication with `## Scope`. Existing anchors
  and every existing `spec_ref` keep resolving.
- **B.** Restructure the normative sections to live under `## Potential
  Features`. Removes duplication but rewrites existing anchors and breaks the
  `spec_ref` of at least T-019, T-101, T-102, T-104, T-105, T-106.

Pick A unless there is a strong reason not to. If B is chosen, repoint every
affected task with `taskrail task repoint <id> --area <anchor>` (dry-run first),
never by hand-editing the `spec_ref` field.

Candidate areas, derived from the existing `## Scope` and `## Changes from
v0.1.0` sections — confirm against the spec rather than adopting verbatim:

- `copy+patch` workspace mode
- `repo-rw` workspace mode
- `--workspace worktree|copy+patch|repo-rw` selection
- `--resume <run-ref>` and same-mode resume rules
- workspace-specific promote behavior
- the unsafe workspace path (`--unsafe-workspace`, `--unsafe`)
- resume-specific evidence additions

Depends on [T-123-rename-specs-to-taskrail-layout](T-123-rename-specs-to-taskrail-layout.md),
because `spec show --anchors` and `coverage --area` are needed to verify this and
do not work under the current filenames.

## Acceptance

- `specs/v0.2.0.md` contains a `## Potential Features` section with one `###`
  heading per agreed feature area.
- `taskrail coverage --json` against v0.2.0 as the active spec reports
  `coverable_areas` equal to the number of `###` areas added, and a non-null
  `coverage_percent`.
- `taskrail coverage --area <anchor>` succeeds for every new area rather than
  returning `is not an area of specs/v0.2.0.md`.
- Every open v0.2.0 task (T-019, T-101, T-102, T-104, T-105, T-106, T-122) still
  has a resolvable `spec_ref` after the change. Under option B they are repointed
  with `taskrail task repoint`, and `taskrail coverage --json` lists no
  unexpected orphans.
- `taskrail-decompose` is demonstrably functional: `coverage --json` returns at
  least one `areas[]` entry, so the skill has a real selection set.
- Normative v0.2.0 behavior is unchanged. Under option A the diff is purely
  additive; under option B no requirement text is altered, only relocated.
- `specs/v0.1.0.md` and `specs/v0.3.0.md` are untouched.
- `taskrail validate` returns `state valid`; `task workflow:validate` and
  `task workflow:verify:spec` pass; the CI planning lane is green.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
