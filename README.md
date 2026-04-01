<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo/lockup-horizontal-mono-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="assets/logo/lockup-horizontal-mono-light.svg">
  <img alt="Tessariq" src="assets/logo/lockup-horizontal-mono-dark.svg" height="64">
</picture>

[![CI](https://github.com/tessariq/tessariq/actions/workflows/ci.yml/badge.svg)](https://github.com/tessariq/tessariq/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/tessariq/tessariq)](https://github.com/tessariq/tessariq/blob/main/go.mod)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](https://github.com/tessariq/tessariq/blob/main/LICENSE)

Tessariq is a Git-native, sandboxed way to run coding agents against a repository.

The product is designed around a simple local workflow:

1. `run` a task against a repo in an isolated workspace
2. `attach` if you want to inspect or interact with the live session
3. review the evidence Tessariq captured
4. `promote` the result into exactly one commit

The core idea is to make agent execution reproducible, inspectable, and usable in normal Git-based development.

## Status

This repository is currently spec-first. The current target release is `v0.1.0`, and the next planned release is `v0.2.0`.

- The current CLI includes `init`, `run`, `attach`, and `promote` for the core detached workflow.
- `v0.1.0` focuses on proving the core local workflow with one workspace model.
- `v0.2.0` expands Tessariq to the required additional workspace modes and same-mode resume behavior.
- Later versions are expected to add operator and admin commands such as `inspect`, `logs`, and `doctor`.

## Product direction

Tessariq is being designed around a few stable principles:

- Git-native workflow and outputs
- detached-by-default execution
- evidence-first runs
- safe-by-default behavior, with explicit unsafe escape hatches only where needed

## Current release path

### v0.1.0

The first release is intended to validate:

- the detached `run -> attach if needed -> promote` flow
- durable evidence artifacts for debugging and future automation
- `worktree` as the default workspace model
- proxy-based egress control for agent runs

### v0.2.0

The second release extends the core model with:

- `copy+patch`
- `repo-rw`
- same-mode `resume` within each workspace mode

## Repository layout

```text
<repo>/
  README.md
  specs/
  .tessariq/
```

- `specs/` is the canonical home for the versioned product specs at the repository root.
- `.tessariq/` is expected to be generated runtime state at the repository root once the implementation exists; it is not the home for repo-authored specs.

## Read next

Start here:

1. `specs/tessariq-v0.1.0.md`
2. `specs/tessariq-v0.2.0.md`
3. `specs/README.md`

## Notes

- This README is an orientation document, not the normative source of truth.
- The versioned specs in `specs/` define the actual release scope and behavior.
