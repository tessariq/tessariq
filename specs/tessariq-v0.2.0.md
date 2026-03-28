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

## Inheritance from v0.1.0

v0.2.0 inherits all v0.1.0 behavior unless this document changes it explicitly. In particular, these invariants still hold:

- runs remain detached by default
- evidence remains durable and repo-local
- promotion remains the normal path from isolated workspace output into Git history
- `promote` still creates exactly one commit or fails cleanly
- evidence JSON artifacts remain parseable under the same compatibility rules

This document focuses on the additions and changed guarantees for multi-workspace operation and resume, but it also includes the implementation notes needed to read the file on its own.

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

## Workspace guarantees

| Workspace | Host repo mutated during `run` | Reproducibility | Unsafe opt-in required | Resume basis | Promote path |
| --- | --- | --- | --- | --- | --- |
| `worktree` | No | Strong, from resume base on a clean repo | No | latest committed state inside prior worktree | Commit from isolated workspace output |
| `copy+patch` | No | Strong, from original `base_sha` plus cumulative patch | No | original `base_sha` plus prior `diff.patch` | Apply patch to fresh isolated checkout, then commit |
| `repo-rw` | Yes | Unsafe and non-reproducible | Yes | current repository working directory state | Commit directly from repository working tree |

### `worktree`

`worktree` behavior from v0.1.0 remains unchanged except that it can now be resumed.

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

### `repo-rw`

Intent:

- provide a local debugging and escape-hatch mode that edits the repository working tree directly

Required behavior:

- mount the repository read-write at `/work`
- require `--unsafe-workspace` or `--unsafe`
- print a warning before run start
- record the unsafe mode in `manifest.json`

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
- `resume` MUST fail if the referenced run is unknown or lacks the required reconstruction evidence for its workspace mode

### `tessariq promote <run-ref>`

The high-level contract remains unchanged:

- create a branch
- create exactly one commit
- default trailers still apply

v0.2.0 adds workspace-specific promote semantics.

## Resume rules

### General rules

- `resume` always creates a new `run_id`
- `resume` never overwrites earlier evidence
- `manifest.json` and `workspace.json` MUST record `resume_from`
- runs in any finished state MAY be resumed if their workspace-specific reconstruction inputs still exist
- live runs MUST NOT be resumed

### Workspace-specific resume behavior

| Workspace | How the resumed workspace is constructed | Required failure behavior |
| --- | --- | --- |
| `worktree` | determine `resume_base_sha` from the previous worktree `HEAD`, then create a new detached worktree there | fail if the old worktree no longer exists or its Git state cannot be read |
| `copy+patch` | create a fresh checkout at the original run's `base_sha`, then apply the old `diff.patch` | fail if `diff.patch` is missing or cannot be applied cleanly |
| `repo-rw` | use the repository's current working directory state as `/work` | warn that the resumed run is non-reproducible; fail if unsafe workspace opt-in is absent |

`copy+patch` resumed runs MUST generate a cumulative `diff.patch` against the same original `base_sha`.

## Promote rules by workspace

### `worktree`

- unchanged from v0.1.0

### `copy+patch`

- create a fresh detached worktree at `base_sha`
- apply `diff.patch`
- fail cleanly if the patch cannot be applied
- create the branch
- create exactly one commit

### `repo-rw`

- create the branch in the repository working tree
- use `git add -A`
- create exactly one commit
- print a warning that the commit was produced from an unsafe workspace mode

For all workspace modes:

- zero-diff promote MUST fail without creating a branch or commit
- promote MUST fail if the run is unfinished, unknown, or missing required evidence for its workspace mode

## Lifecycle rules

| Action | Valid source state | Success result | Required failure behavior |
| --- | --- | --- | --- |
| `run` | clean repo for `worktree` and `copy+patch`; explicit unsafe opt-in for `repo-rw` | new run with a workspace-specific evidence set | fail if workspace safety preconditions are not met |
| `attach` | live run only | terminal attached to live `tmux` session | unchanged from v0.1.0 |
| `resume` | finished run with reconstructable workspace inputs | new run continuing from prior workspace state | fail if source run is live, unknown, or missing required inputs |
| `promote` | finished run with code changes and required evidence | one branch and exactly one commit | fail if run is unfinished, unknown, zero-diff, or cannot be reconstructed |

## Evidence additions

v0.2.0 keeps the v0.1.0 evidence contract and extends it.

Required `workspace.json` fields for all workspace modes:

```json
{
  "schema_version": 1,
  "workspace_mode": "copy+patch",
  "base_sha": "abc123",
  "repo_mount_mode": "ro",
  "resume_from": "01J...",
  "reproducibility": "strong"
}
```

Required `manifest.json` additions when relevant:

```json
{
  "schema_version": 1,
  "workspace_mode": "copy+patch",
  "resume_from": "01J...",
  "unsafe_workspace": false
}
```

Rules:

- `copy+patch` MUST emit `diff.patch` and `diffstat.txt` when there are changes
- patch generation for `copy+patch` MUST be deterministic
- `repo-rw` evidence MUST make its weaker guarantees explicit
- `resume_from` MUST refer to the immediately resumed prior `run_id`, not the root ancestor

## Acceptance scenarios

- all three workspace modes run end to end
- `copy+patch` produces a deterministic patch and promotes it into one commit
- `repo-rw` requires explicit unsafe opt-in and records that choice in evidence
- `resume` works for `worktree`, `copy+patch`, and `repo-rw`
- resumed `copy+patch` runs generate cumulative patches against a stable `base_sha`
- resuming a live run fails cleanly
- resuming a run with missing reconstruction inputs fails cleanly
- all workspace-specific warnings and evidence fields are present

## Implementation Notes (Informative)

This section is informative. It describes the current implementation shape for v0.2.0, and the normative sections above take precedence if there is any conflict.

### Generated storage layout

```text
<repo>/
  specs/
  .tessariq/
    runs/
      index.jsonl
      <run_id>/
        manifest.json
        status.json
        adapter.json
        task.md
        run.log
        runner.log
        diff.patch
        diffstat.txt
        egress.compiled.yaml
        egress.events.jsonl
        squid.log
        timeout.flag
        bootstrap.sh
        runner.sh
        workspace.json
```

```text
~/.tessariq/
  worktrees/
    <repo_id>/
      <run_id>/
```

### Derived identifiers

- `run_id` is a ULID
- `repo_root = realpath(git rev-parse --show-toplevel)`
- `repo_id = slug(basename(repo_root)) + "-" + shortHash(repo_root)`
- `shortHash` is the first 8 hex chars of `sha256(repo_root)`

### Shared runtime sketch

The current implementation direction shared across workspace modes is:

- keep one evidence folder per run under `.tessariq/runs/<run_id>/`
- keep detached worktrees under `~/.tessariq/worktrees/<repo_id>/<run_id>/` when the workspace mode needs them
- use the same proxy topology as v0.1.0 for `proxy` egress:
  - create a per-run internal `run_net`
  - start a per-run Squid proxy container connected to `run_net` and a non-internal egress network
  - run the agent container only on `run_net`
  - configure `HTTP_PROXY` and `HTTPS_PROXY` for the agent

### Workspace-specific implementation notes

`worktree`:

- continue using a detached host worktree mounted read-write at `/work`
- resume by reading the previous worktree `HEAD` and creating a fresh detached worktree from that point

`copy+patch`:

- construct `/work` from a deterministic checkout at `base_sha`
- generate `diff.patch` and `diffstat.txt` from the in-container working copy
- resume by re-checking out the original `base_sha` and applying the prior `diff.patch`
- promote by applying `diff.patch` to a fresh detached worktree before branching and committing

`repo-rw`:

- mount the repository working tree directly at `/work`
- record the unsafe workspace choice in evidence
- resume from the repository's current working directory state rather than a reconstructable isolated snapshot

### Runner responsibilities

Runner, as PID1, is expected to:

- start the `tmux` session
- enforce timeout
- write `timeout.flag` before escalation on timeout
- ensure `status.json` exists even if bootstrap fails
- write `runner.log`

### Bootstrap responsibilities

Bootstrap is expected to:

- run `pre` commands
- run the selected adapter
- run `verify` commands
- trap `EXIT`
- generate diff artifacts best-effort
- write the final `status.json`
