# Changelog Policy

How `CHANGELOG.md` entries are written in this repository. The checklists in
`AGENTS.md`, `CONTRIBUTING.md`, and `development-workflow.md` link here instead
of restating the policy.

## When to add an entry

- Add an entry under `## [Unreleased]` for user-visible behavior changes only.
- Skip internal-only refactors, CI plumbing, test-only work, and routine
  dependency bumps with no user-visible effect.
- Fold one user-facing change into one entry even when it spans several tasks.
- Put the entry under the Keep a Changelog section that describes its effect:
  `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, or `Security`.

## How to write it

- Keep entries terse: one or two sentences.
- Lead with the command or observable behavior and state what changed for the
  operator, including flags they type when relevant.
- Leave out function names, internal schemas, implementation mechanics, test
  strategy, and design rationale. Those belong in the commit body or spec.
- Describe security boundaries precisely without turning the entry into a
  threat-model essay.
- Copy-edit against the v0.1.0 entries so tone and length stay consistent.

## Examples

- Good: `` `promote` now rejects inconsistent run identity evidence before it
  creates a branch or commit. ``
- Bad: a paragraph naming internal functions, every compared field, the test
  layers, and the implementation history behind the change.

## Preparing a release

Release notes are extracted from the matching `## [<version>]` section. Before
tagging, move Unreleased entries under `## [<version>] - YYYY-MM-DD`, leave a new
empty `## [Unreleased]` section above it, and update the comparison links at the
end of `CHANGELOG.md`.
