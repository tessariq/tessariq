# Tessariq v0.2.0 Specification

**Status:** Draft (normative)  
**Scope:** Second release  
**Theme:** Expand Tessariq to the full planned workspace model

## Release intent

Tessariq v0.2.0 is intended to verify:

- how the required workspace modes are used in practice
- `copy+patch` provides meaningful isolation value over `worktree`
- `repo-rw` is useful enough to justify its weaker safety and reproducibility guarantees
- resume across all workspace modes materially improves iteration speed
- the multi-workspace model can stay coherent without adding the later operator CLI yet

## Product intent

v0.2.0 extends the v0.1.0 core workflow without changing its primary invariants:

- runs remain detached by default
- evidence remains durable and repo-local
- `promote` remains the only standard path to a branchable commit for isolated workspace modes
- the evidence contract remains stable enough for later automation

## Scope

v0.2.0 adds these normative capabilities:

- `copy+patch`
- `repo-rw`
- `--workspace worktree|copy+patch|repo-rw`
- `--resume <run-ref>` for all workspace modes

Still out of scope:

- `inspect`, `logs`, `list`, `stop`, `clean`, `doctor`
- Kubernetes or distributed execution
- multi-agent orchestration
- web UI or database
- automatic push or PR creation

## CLI changes from v0.1.0

### `tessariq run <task-path>`

New flags:

- `--workspace worktree|copy+patch|repo-rw`
- `--unsafe-workspace`
- `--resume <run-ref>`
- `--unsafe` as a convenience flag covering `--unsafe-workspace` and `--unsafe-egress`

Rules:

- default workspace remains `worktree`
- `repo-rw` MUST require explicit unsafe opt-in
- `resume` always creates a new `run_id` and a new evidence folder

### `tessariq promote <run-ref>`

The high-level contract remains unchanged:

- create a branch
- create exactly one commit
- default trailers still apply

v0.2.0 adds workspace-specific promote semantics.

## Workspace modes

### `worktree`

`worktree` behavior from v0.1.0 remains unchanged.

### `copy+patch`

Intent:

- avoid a read-write bind mount of repository code from the host
- preserve a clear promote path back into Git

Required behavior:

- the host repository is mounted read-only for source material only
- the working copy inside the container lives at `/work`
- `/work` MUST be a deterministic Git checkout at `base_sha`, not a raw file copy
- the agent modifies `/work`
- no host-visible working tree changes occur during the run

Required evidence:

- `diff.patch`
- `diffstat.txt`
- `workspace.json` fields:
  - `workspace_mode = "copy+patch"`
  - `base_sha`
  - `repo_mount_mode = "ro"`
  - `resume_from` when applicable
  - `reproducibility = "strong"`

### `repo-rw`

Intent:

- provide a local debugging and escape-hatch mode that edits the repository working tree directly

Required behavior:

- mount the repository read-write at `/work`
- require `--unsafe-workspace` or `--unsafe`
- print a warning before run start
- record the unsafe mode in `manifest.json`

Required evidence:

- `workspace.json` fields:
  - `workspace_mode = "repo-rw"`
  - `base_sha`
  - `repo_mount_mode = "rw"`
  - `resume_from` when applicable
  - `reproducibility = "unsafe"`

## Resume semantics

General rules:

- `resume` always creates a new `run_id`
- `resume` never overwrites earlier evidence
- `manifest.json` and `workspace.json` MUST record `resume_from`

### Resume for `worktree`

- determine `resume_base_sha = git -C <old_worktree> rev-parse HEAD`
- create the new detached worktree at `resume_base_sha`
- proceed as a normal run from that state

### Resume for `copy+patch`

- recreate a fresh checkout at the original run's `base_sha`
- apply the old run's `diff.patch` to reconstruct the resumed workspace
- start the new run from that reconstructed state
- generate the new `diff.patch` as a cumulative patch against the same `base_sha`

Rationale:

- cumulative patches keep promote behavior deterministic
- the resumed run remains representable as one patch against one Git base

### Resume for `repo-rw`

- use the repository's current working directory state as `/work`
- warn that the resumed run is inherently non-reproducible
- record `reproducibility = "unsafe"` in evidence

## Promote semantics by workspace

### Promote for `worktree`

- unchanged from v0.1.0

### Promote for `copy+patch`

- create a fresh detached worktree at `base_sha`
- apply `diff.patch`
- fail cleanly if the patch cannot be applied
- create the branch
- create exactly one commit

### Promote for `repo-rw`

- create the branch in the repository working tree
- use `git add -A`
- create exactly one commit
- print a warning that the commit was produced from an unsafe workspace mode

For all workspace modes:

- zero-diff promote MUST fail without creating a branch or commit

## Evidence additions

v0.2.0 keeps the v0.1.0 evidence contract and extends it.

Additional minimum `workspace.json` fields:

```json
{
  "workspace_mode": "copy+patch",
  "base_sha": "abc123",
  "repo_mount_mode": "ro",
  "resume_from": "01J...",
  "reproducibility": "strong"
}
```

Additional minimum `manifest.json` fields:

```json
{
  "workspace_mode": "copy+patch",
  "resume_from": "01J...",
  "unsafe_workspace": false
}
```

Rules:

- `copy+patch` MUST always emit `diff.patch` and `diffstat.txt` when there are changes
- patch-generation strategy MUST be deterministic
- `repo-rw` evidence MUST make its weaker guarantees explicit

## Acceptance criteria

- all three workspace modes run end to end
- `copy+patch` produces a deterministic patch and promotes it into one commit
- `repo-rw` requires explicit unsafe opt-in and records that choice in evidence
- `resume` works for `worktree`, `copy+patch`, and `repo-rw`
- resumed `copy+patch` runs generate cumulative patches against a stable `base_sha`
- all workspace-specific warnings and evidence fields are present
