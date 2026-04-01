# Tessariq Specs

This folder contains the versioned product specs for Tessariq.

## Reading order

1. Read `tessariq-v0.1.0.md` for the first shippable release.
2. Read `tessariq-v0.2.0.md` for the next release that expands the workspace model.
3. Read `tessariq-v0.3.0.md` for the draft release that adds safe custom runtime baking.

## Why the specs are versioned

- Each version has a clear release intent, so the team can verify whether the release actually taught us what it was supposed to teach.
- Versioned specs force scope discipline. Power-user and operator features should not blur the learning goals of the first release.
- Later versions inherit earlier invariants unless a spec explicitly changes them.

## Versions

### v0.1.0

Reasoning:
- Prove the core product loop before broadening the surface area.
- Validate the detached local workflow: `run -> attach if needed -> promote`.
- Validate evidence quality and proxy-based egress control with the simplest workspace model.
- Validate the agent/runtime split: selected agents, runtime-image override, minimal reference runtime, and read-only auth reuse.

Primary scope:
- `worktree` workspace only
- clean-repo-only execution
- core CLI only: `init`, `version`, `run`, `attach`, `promote`
- first-party agents: `claude-code`, `opencode`
- one official minimal reference runtime image

### v0.2.0

Reasoning:
- The product already knows it needs the other workspace modes.
- The next step is to broaden capability without weakening the core v0.1.0 invariants.
- Resume becomes part of the workspace story, not a disconnected convenience feature.

Primary scope:
- `copy+patch`
- `repo-rw`
- same-mode `resume` within each workspace mode
- per-workspace promote and evidence semantics
- inherited agent/runtime/auth model from v0.1.0

### v0.3.0

Reasoning:
- After the minimal reference runtime proves useful, the next product step is to make safe custom runtime creation easier without turning Tessariq into a third-party agent version manager.
- Tessariq should provide safe runtime-building primitives and provenance, not mirror upstream agent release lifecycles.

Primary scope:
- safe custom runtime baking from an embedded Tessariq Dockerfile
- stronger runtime provenance in `runtime.json`
- runtime validation and documentation for custom image flows

### Later versions

Reasoning:
- Operator and admin commands are useful, but they are not the next validation target.
- Devcontainer-derived runtimes are promising, but they should land only after the basic custom-runtime flow is proven.

Planned later scope:
- devcontainer-derived runtime support
- `inspect`
- `logs`
- `list`
- `stop`
- `clean`
- `doctor`

## Authoring conventions

- Each versioned spec must declare a document `Status` in the header.
- The `Status` line must contain only one of these values: `Draft`, `In Progress`, or `Done`.
- `Draft` means the version is exploratory or planned and is not yet the active implementation baseline.
- `In Progress` means the version is the active implementation baseline and remains normative while delivery is underway.
- `Done` means the version is shipped or otherwise contract-locked; further behavior changes should be introduced in a newer versioned spec.
- Each versioned spec must include a `Release intent` section.
- Specs should state both what is in scope and what is intentionally deferred.
- Versioned specs should optimize for user-visible contracts, guarantees, and acceptance scenarios rather than runtime internals.
- Each versioned spec may include an `Implementation Notes (Informative)` section for version-specific runtime detail.
- Informative sections describe likely implementation shape, but the normative sections in the same file define release behavior.
- Repo-tracked Markdown belongs in `specs/` at the repository root; generated runtime state belongs in `<repo>/.tessariq/`.

## Terminology

- `Status` refers to spec-document maturity in the document header.
- `run state` or `run outcome` refers to Tessariq runtime behavior such as `status.json` and lifecycle transitions.
- `workspace mode` refers to how Tessariq exposes repository contents to a run, such as `worktree`, `copy+patch`, or `repo-rw`.
- `evidence artifact` refers to a durable file emitted under `<repo>/.tessariq/runs/<run_id>/`.
- `agent` refers to the selected coding tool Tessariq invokes, such as `claude-code` or `opencode`.
- `runtime` refers to the container execution environment for a run.
- `runtime image` refers to the Docker image used to provide that runtime.
