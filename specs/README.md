# Tessariq Specs

This folder contains the versioned product and technical specs for Tessariq.

## Reading order

1. Read `tessariq-v0.1.0.md` for the first shippable release.
2. Read `tessariq-v0.2.0.md` for the next release that expands the workspace model.

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

Primary scope:
- `worktree` workspace only
- clean-repo-only execution
- core CLI only: `init`, `run`, `attach`, `promote`
- adapters: `claude-code`, `opencode`

### v0.2.0

Reasoning:
- The product already knows it needs the other workspace modes.
- The next step is to broaden capability without weakening the core v0.1.0 invariants.
- Resume becomes part of the workspace story, not a disconnected convenience feature.

Primary scope:
- `copy+patch`
- `repo-rw`
- `resume` across all workspace modes
- per-workspace promote and evidence semantics

### Later versions

Reasoning:
- Operator and admin commands are useful, but they are not the next validation target.
- They should be planned after the multi-workspace model is proven in practice.

Planned later scope:
- `inspect`
- `logs`
- `list`
- `stop`
- `clean`
- `doctor`

## Authoring conventions

- Each versioned spec must include a `Release intent` section.
- Specs should state both what is in scope and what is intentionally deferred.
- Repo-tracked Markdown belongs in `specs/`; generated runtime state belongs in `.tessariq/`.
