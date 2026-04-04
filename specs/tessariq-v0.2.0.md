# Tessariq v0.2.0 Specification

**Status:** Draft  
**Scope:** Second release  
**Theme:** Expand Tessariq to the full planned workspace model

## Release intent

Tessariq v0.2.0 is intended to verify:

- how the required workspace modes are used in practice
- `copy+patch` provides meaningful isolation value over `worktree`
- `repo-rw` is useful enough to justify its weaker safety and reproducibility guarantees
- same-mode resume for each workspace materially improves iteration speed
- the multi-workspace model can stay coherent without adding the later operator CLI yet
- the v0.1.0 agent/runtime/auth contract remains stable across all workspace modes

## Inheritance from v0.1.0

v0.2.0 inherits all v0.1.0 behavior unless this document changes it explicitly. In particular, these invariants still hold:

- runs remain detached by default
- evidence remains durable and repo-local
- promotion remains the normal path from isolated workspace output into Git history
- `promote` still creates exactly one commit or fails cleanly
- evidence JSON artifacts remain parseable under the same compatibility rules
- host prerequisite guarantees and missing-prerequisite failure UX from v0.1.0 remain in force
- the user-facing tool choice remains `agent`, not `adapter`
- the selected runtime image remains a separate concept from the selected agent
- supported-agent auth reuse remains read-only and MUST NOT expose host `HOME`
- `--mount-agent-config` remains an opt-in read-only config-dir mount for supported agents

This document focuses on the additions and changed guarantees for multi-workspace operation and resume, but it also includes the implementation notes needed to read the file on its own.

## Scope

v0.2.0 adds these normative capabilities:

- `copy+patch`
- `repo-rw`
- `--workspace worktree|copy+patch|repo-rw`
- `--resume <run-ref>` within each workspace mode

Still out of scope:

- `inspect`, `logs`, `list`, `stop`, `clean`, `doctor`
- Kubernetes or distributed execution
- multi-agent orchestration
- web UI or database
- automatic push or PR creation
- tracking or pinning upstream third-party agent versions as a Tessariq product responsibility
- devcontainer-derived runtime support
- writable host credential or config mounts for agent auth refresh

`clean` remains intentionally out of scope for v0.2.0. Users may still remove retained Tessariq-generated local state manually when they no longer need it for inspection, resume, or promote, and a first-class cleanup or prune command is deferred to a later release.

## Changes from v0.1.0

v0.2.0 extends or overrides v0.1.0 in these areas:

- add `copy+patch` and `repo-rw` workspace modes
- add `--workspace worktree|copy+patch|repo-rw`
- add `--resume <run-ref>` with same-mode resume and resume-specific evidence fields
- add `--unsafe-workspace` and `--unsafe` for the unsafe workspace path
- add workspace-specific promote behavior for `copy+patch` and `repo-rw`
- retain the v0.1.0 run outcome model; this spec changes workspace and resume behavior, not terminal run outcomes
- keep the v0.1.0 agent/runtime/auth/egress model unchanged while broadening workspace choice

## Workspace guarantees

| Workspace | Host repo mutated during `run` | Reproducibility | Unsafe opt-in required | Resume basis | Promote path |
| --- | --- | --- | --- | --- | --- |
| `worktree` | No | Strong, from preserved prior workspace state on a clean repo | No | preserved workspace snapshot from the source run | Commit from isolated workspace output |
| `copy+patch` | No | Strong, from original `base_sha` plus cumulative patch | No | original `base_sha` plus prior `diff.patch` | Apply patch to fresh isolated checkout, then commit |
| `repo-rw` | Yes | Unsafe and non-reproducible, but promote attribution is bounded by a captured baseline | Yes | current repository working directory state plus the lineage baseline snapshot | Commit the net delta from the captured lineage baseline |

### `worktree`

`worktree` behavior from v0.1.0 remains unchanged except that it can now be resumed.

Resume-specific behavior:

- Tessariq MUST preserve the source run's in-progress workspace state, including uncommitted tracked changes and untracked non-ignored files
- resume MUST reconstruct the new isolated workspace from that preserved state rather than from the source worktree `HEAD`
- the preserved reconstruction material MUST be retained until successful `promote` or explicit cleanup

### `copy+patch`

Intent:

- avoid a read-write bind mount of repository code from the host
- preserve a clear promote path back into Git

Required behavior:

- Tessariq MUST refuse to start a `copy+patch` run if the repository has staged, unstaged, or untracked non-ignored files
- the host repository is mounted read-only for source material only
- the working copy inside the container lives at `/work`
- `/work` MUST be a deterministic Git checkout at `base_sha`, not a raw file copy
- the agent modifies `/work`
- no host-visible working tree changes occur during the run
- dirty-repo failure for `copy+patch` MUST happen before container start and tell the user to commit, stash, or clean the repository first

### `repo-rw`

Intent:

- provide a local debugging and escape-hatch mode that edits the repository working tree directly

Required behavior:

- mount the repository read-write at `/work`
- require `--unsafe-workspace` or `--unsafe`
- print a warning before run start
- record the unsafe mode in `manifest.json`
- capture a run-start baseline snapshot before agent-visible mutation begins, even when the repository starts dirty
- retain the lineage baseline needed for future `resume` and `promote` until successful `promote` or explicit cleanup

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
- `--unsafe` acts as a convenience flag only; it does not select `repo-rw` by itself and does not override an explicit `--workspace` or `--egress` value
- `--unsafe-workspace` satisfies the workspace safety gate only for `--workspace repo-rw`
- `--unsafe-egress` remains an alias for `--egress open` as inherited from v0.1.0
- `resume` always creates a new `run_id` and a new evidence folder
- if `--resume` is set without `--workspace`, the resumed run MUST default to the source run's workspace mode
- if both `--resume` and `--workspace` are set, the requested workspace mode MUST match the source run's workspace mode or Tessariq MUST fail before container start
- `resume` MUST fail if the referenced run is unknown or lacks the required reconstruction evidence for its workspace mode
- the inherited `--agent`, `--image`, and `--mount-agent-config` contracts remain in force for resumed runs

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
- `resume` is same-mode only; cross-workspace resume is out of scope for v0.2.0
- in this spec, a "finished run" means a run in one of the inherited v0.1.0 terminal run outcomes: `success`, `failed`, `timeout`, `killed`, or `interrupted`
- runs in any finished state MAY be resumed if their workspace-specific reconstruction inputs still exist
- live runs MUST NOT be resumed
- reconstructable workspace material and lineage baselines MUST be retained until successful `promote` or explicit cleanup

### Workspace-specific resume behavior

| Workspace | How the resumed workspace is constructed | Required failure behavior |
| --- | --- | --- |
| `worktree` | restore the preserved workspace snapshot from the source run into a new detached worktree | fail if the preserved workspace snapshot is missing or cannot be restored cleanly |
| `copy+patch` | create a fresh checkout at the original run's `base_sha`, then apply the old `diff.patch` | fail if `diff.patch` is missing or cannot be applied cleanly |
| `repo-rw` | use the repository's current working directory state as `/work`, carrying forward the original lineage baseline for later promote | warn that the resumed run is non-reproducible; fail if unsafe workspace opt-in is absent or the lineage baseline is missing |

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

- compare the repository working tree against the captured lineage baseline rather than blindly promoting the entire current tree
- fail cleanly if the lineage baseline cannot be reconciled or if repository `HEAD` changed since the lineage root run started
- create the branch in the repository working tree
- use `git add -A` for the net delta selected from that comparison
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
  "lineage_root_run_id": "01J...",
  "reproducibility": "strong"
}
```

Required `manifest.json` additions when relevant:

```json
{
  "schema_version": 1,
  "workspace_mode": "copy+patch",
  "resume_from": "01J...",
  "lineage_root_run_id": "01J...",
  "unsafe_workspace": false
}
```

Rules:

- `copy+patch` MUST emit `diff.patch` and `diffstat.txt` when there are changes
- patch generation for `copy+patch` MUST be deterministic
- `repo-rw` evidence MUST make its weaker guarantees explicit
- `resume_from` MUST refer to the immediately resumed prior `run_id`, not the root ancestor
- `lineage_root_run_id` MUST refer to the first run in the promote lineage
- `worktree` evidence MUST describe the preserved workspace snapshot needed for same-mode resume
- `repo-rw` evidence MUST describe the captured lineage baseline used to bound promote attribution
- the inherited `agent.json` and `runtime.json` artifacts remain required for all workspace modes

## Acceptance scenarios

- all three workspace modes run end to end
- `copy+patch` produces a deterministic patch and promotes it into one commit
- `repo-rw` requires explicit unsafe opt-in and records that choice in evidence
- `resume` works for `worktree`, `copy+patch`, and `repo-rw`
- `worktree` resume preserves uncommitted tracked changes and untracked non-ignored files
- resumed `copy+patch` runs generate cumulative patches against a stable `base_sha`
- `--resume` with a different `--workspace` fails before container start
- `repo-rw` may start dirty but `promote` commits only the net delta from the captured lineage baseline
- `repo-rw` promote fails cleanly if repository `HEAD` changed after the lineage root run started
- resuming a live run fails cleanly
- resuming a run with missing reconstruction inputs fails cleanly
- missing or unavailable host prerequisites fail cleanly with actionable guidance
- inherited supported-agent auth reuse still works across all workspace modes
- inherited `--mount-agent-config` behavior still works across all workspace modes
- inherited `auto` egress remains agent-aware across all workspace modes
- all workspace-specific warnings and evidence fields are present

## Failure UX

| Condition | Required behavior | Required user guidance |
| --- | --- | --- |
| `--resume` references a live run | fail before container start | tell the user the source run is still live and must finish before resume |
| required host prerequisite (`git`, `tmux`, or `docker`) is missing or unavailable | fail before dependent command work begins | identify which prerequisite is missing or unavailable and tell the user to install or enable it, then retry |
| `--resume` requests a different workspace mode | fail before container start | print both workspace modes and tell the user v0.2 resume is same-mode only |
| `worktree` resume reconstruction material is missing | fail before container start | identify the missing preserved workspace state and tell the user resume is no longer possible |
| `copy+patch` reconstruction patch is missing or cannot be applied | fail before container start or promote | identify the patch failure and tell the user the run cannot be resumed or promoted cleanly |
| `repo-rw` is selected without `--unsafe-workspace` or `--unsafe` | fail before container start | explain that `repo-rw` is unsafe and requires explicit opt-in |
| `repo-rw` lineage baseline is missing or repository `HEAD` changed since the lineage root run started | fail without creating a branch or commit | tell the user promote attribution is no longer safe and the run must be rerun from a fresh baseline |
| the selected agent binary is missing from the resolved runtime image | fail before agent start | unchanged from v0.1.0 |
| required supported agent auth state is missing or requires writable refresh | fail before agent start | unchanged from v0.1.0 |
| `promote` sees zero diff | fail without creating a branch or commit | tell the user there were no code changes to promote |

## Success metrics

- at least 85% of finished runs with resume-eligible evidence can be resumed successfully in the same workspace mode
- at least 80% of resumed `worktree` runs preserve in-progress state without user-reported loss of edits
- at least 75% of `copy+patch` runs with code changes promote successfully without patch-apply failure
- at least 90% of `repo-rw` promote attempts either produce a bounded single commit or fail with the defined attribution-safety message
- fewer than 10% of resume failures are caused by premature loss of retained reconstruction material

## Implementation Notes (Informative)

This section is informative. It describes the current implementation shape for v0.2.0, and the normative sections above take precedence if there is any conflict.

### Generated storage layout

Repo-local generated state lives at the repository root under `<repo>/.tessariq/`, alongside `specs/`.

```text
<repo>/
  specs/
  .tessariq/
    runs/
      index.jsonl
      <run_id>/
        manifest.json
        status.json
        agent.json
        runtime.json
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

Detached worktrees, when needed, live outside the repository under the user's home directory:

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

- keep one evidence folder per run under `<repo>/.tessariq/runs/<run_id>/` at the repository root
- keep detached worktrees under `~/.tessariq/worktrees/<repo_id>/<run_id>/` when the workspace mode needs them
- use the same proxy topology as v0.1.0 for `proxy` egress:
  - create a per-run internal `run_net`
  - start a per-run Squid proxy container connected to `run_net` and a non-internal egress network
  - run the agent container only on `run_net`
  - configure `HTTP_PROXY` and `HTTPS_PROXY` for the agent
- keep the same read-only supported-agent auth reuse and optional read-only `--mount-agent-config` behavior from v0.1.0

### Workspace-specific implementation notes

`worktree`:

- continue using a detached host worktree mounted read-write at `/work`
- resume from preserved workspace reconstruction material rather than from the previous worktree `HEAD`

`copy+patch`:

- construct `/work` from a deterministic checkout at `base_sha`
- generate `diff.patch` and `diffstat.txt` from the in-container working copy
- resume by re-checking out the original `base_sha` and applying the prior `diff.patch`
- promote by applying `diff.patch` to a fresh detached worktree before branching and committing

`repo-rw`:

- mount the repository working tree directly at `/work`
- record the unsafe workspace choice in evidence
- capture and retain a lineage baseline snapshot to bound later promote attribution
- resume from the repository's current working directory state while carrying forward that lineage baseline

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
- run the selected agent
- run `verify` commands
- trap `EXIT`
- generate diff artifacts best-effort
- write the final `status.json`
