# Tessariq v0.1.0 Specification

**Status:** Draft (normative)  
**Scope:** First release  
**Theme:** Prove the core local workflow before expanding workspace breadth

## Release intent

Tessariq v0.1.0 is intended to verify:

- developers want a detached-by-default local workflow for agent runs
- `run -> attach if needed -> promote` is understandable and fast enough in daily use
- durable evidence artifacts are sufficient for debugging and future automation
- `worktree` is a good default balance of UX, Git integration, and safety
- proxy-based egress control is practical for real agent usage without breaking developer UX

## Product intent

Tessariq v0.1.0 provides a Git-native, sandboxed way to run coding agents against a repository. The release is centered on four user-visible contracts:

- runs are detached by default, with optional attach to a live `tmux` session
- the default workspace is isolated from the repository working tree
- evidence is durable, repo-local, and stable enough for later automation
- promotion produces exactly one reviewable Git commit or fails cleanly

Implementation detail that is useful for building Tessariq but not needed to understand release scope lives in [runtime-design-notes.md](/media/felix/data/code/tessariq/specs/runtime-design-notes.md).

## Goals

1. Excellent local developer UX.
2. Stable run identity, lifecycle, and evidence layout.
3. Safe-by-default networking, with explicit unsafe opt-in for `open`.
4. Evidence contracts that later automation can parse without guessing.

## Non-goals

- `copy+patch`
- `repo-rw`
- `resume`
- `inspect`, `logs`, `list`, `stop`, `clean`, `doctor`
- Kubernetes or distributed execution
- multi-agent orchestration
- web UI or database
- automatic push or PR creation

## Core workflow

### Run

A run is identified by a ULID `run_id` and produces:

- one isolated workspace
- one evidence folder under `<repo>/.tessariq/runs/<run_id>/`
- one terminal lifecycle
- one final state: `success|failed|timeout|killed|interrupted`

### Promote

`promote` converts a run output into:

- one branch
- exactly one commit containing code changes
- commit trailers linking the commit back to the run by default

Promotion is the normal path from isolated workspace output into ordinary Git review flow.

## Repository model

### User-authored inputs

- `tessariq run <task-path>` accepts a Markdown file inside the current repository
- Tessariq copies the exact task file into evidence as `task.md`
- the first Markdown H1, if present, becomes `task_title`
- if no H1 exists, `task_title` falls back to the task file basename without extension

### Generated runtime state

- runtime state is generated under `<repo>/.tessariq/`
- `tessariq init` MUST add `.tessariq/` to `.gitignore`
- repo-tracked config files are out of scope for v0.1.0; behavior is driven by CLI flags

## Workspace guarantees

v0.1.0 supports exactly one workspace mode: `worktree`.

| Workspace | Host repo mutated during `run` | Reproducibility | Unsafe opt-in required | Promote path |
| --- | --- | --- | --- | --- |
| `worktree` | No | Strong, from `base_sha` on a clean repo | No | Commit from isolated workspace output |

Required `worktree` behavior:

- Tessariq creates a detached worktree for the run
- the container mounts that worktree read-write at `/work`
- evidence is mounted separately from `/work`
- `base_sha` is the repository `HEAD` at run start
- Tessariq MUST refuse to start a run if the repository has staged, unstaged, or untracked non-ignored files
- dirty-repo failure MUST happen before container start and tell the user to commit, stash, or clean the repository first

## CLI

### `tessariq init`

Creates the local skeleton:

- `specs/`
- `.tessariq/runs/`

It MUST add `.tessariq/` to `.gitignore`.

### `tessariq run <task-path>`

Detached by default.

Defaults:

- `--timeout=30m`
- `--grace=30s`
- `--agent=claude-code`
- `--egress=auto`
- `--attach=false`

Supported flags:

- `--agent claude-code|opencode`
- `--image <image>`
- `--model <string>`
- `--yolo`
- `--egress none|proxy|open|auto`
- `--unsafe-egress` as an alias for `--egress open`
- `--egress-allow <host[:port]>` repeatable
- `--egress-allow-reset`
- `--pre "<cmd>"` repeatable
- `--verify "<cmd>"` repeatable
- `--attach`

Required printed output:

- `run_id`
- workspace path
- evidence path
- container name or id
- attach command
- promote command

### `tessariq attach <run-ref>`

- attaches the user terminal to the run's `tmux` session
- MUST fail cleanly if the run is not live
- failure output MUST include the evidence path

### `tessariq promote <run-ref>`

- MUST create a branch
- MUST create exactly one commit
- MUST use `git add -A`
- MUST include these commit trailers by default:
  - `Tessariq-Run: <run_id>`
  - `Tessariq-Base: <base_sha>`
  - `Tessariq-Task: <task_path>`

Flags:

- `--branch <name>`
- `--message <msg>`
- `--no-trailers`

Defaults:

- branch name: `tessariq/<run_id>`
- commit message: `task_title`
- fallback commit message: `tessariq: apply run <run_id>`

Zero-diff behavior:

- if the run has no code changes, `promote` MUST fail without creating a branch or commit

## Lifecycle rules

| Action | Valid source state | Success result | Required failure behavior |
| --- | --- | --- | --- |
| `run` | clean repository | new run with evidence and a final state | fail before container start if repo is dirty or task path is invalid |
| `attach` | live run only | terminal attached to live `tmux` session | fail if run is finished or unknown; include evidence path when known |
| `promote` | finished run with code changes | one branch and exactly one commit | fail if run is unknown, unfinished, missing required evidence, or has zero diff |

`run-ref` resolution MUST support:

- explicit `run_id`
- `last`
- `last-N`

Resolution is against the current repository's run index. Commands MUST fail if the referenced run cannot be found in that repository.

## Adapter contract

v0.1.0 supports exactly these first-party adapters:

- `claude-code`
- `opencode`

Common rules:

- each adapter MUST write `adapter.json`
- `adapter.json` MUST record requested adapter options
- if an option such as `--model` or `--yolo` cannot be applied exactly, the adapter MUST record that it was requested but not applied

Minimum `adapter.json` shape:

```json
{
  "schema_version": 1,
  "adapter": "claude-code",
  "image": "example/image:tag",
  "requested": {
    "model": "gpt-5.4",
    "yolo": true
  },
  "applied": {
    "model": false,
    "yolo": true
  }
}
```

## Networking and egress

Modes:

- `none`
- `proxy`
- `open`
- `auto`

`auto` resolution:

- for `claude-code` and `opencode`, `auto` MUST resolve to `proxy`
- the resolved mode MUST be written into `manifest.json`

User-visible `proxy` contract:

- the agent only receives network egress through an allowlisted proxy path
- allowlists are enforced at destination `host:port`
- default port is TCP `443`
- HTTPS and WSS use CONNECT tunneling
- URL-path filtering is out of scope for v0.1.0

`open` contract:

- MUST require explicit opt-in
- MUST be recorded in `manifest.json`

Low-level proxy topology and process details are implementation notes, not part of the release contract.

## Evidence contract

### Required artifacts

The following files MUST exist for every run unless marked otherwise:

- `manifest.json`
- `status.json`
- `adapter.json`
- `task.md`
- `run.log`
- `runner.log`
- `workspace.json`
- `diff.patch` when there are changes
- `diffstat.txt` when there are changes
- `egress.compiled.yaml` only in proxy mode
- `egress.events.jsonl` only in proxy mode
- `squid.log` optional and capped

Logs MUST be capped and MUST include a truncation marker if truncated.

### Compatibility rules

- every emitted JSON artifact defined by this spec MUST include `schema_version`
- v0.1.0 defines `schema_version: 1` for `manifest.json`, `status.json`, `adapter.json`, and `workspace.json`
- required fields in the minimum shapes below MUST always be present
- implementations MAY add extra fields without changing `schema_version` if they do not change the meaning of existing fields
- `index.jsonl` is append-only; each line represents one run and MUST not be rewritten in place for another run

Minimum `manifest.json` shape:

```json
{
  "schema_version": 1,
  "run_id": "01J...",
  "task_path": "specs/example.md",
  "task_title": "Example task",
  "adapter": "claude-code",
  "base_sha": "abc123",
  "workspace_mode": "worktree",
  "requested_egress_mode": "auto",
  "resolved_egress_mode": "proxy",
  "container_name": "tessariq-01J...",
  "created_at": "2026-01-27T12:00:00Z"
}
```

Minimum `status.json` shape:

```json
{
  "schema_version": 1,
  "state": "success",
  "started_at": "2026-01-27T12:00:00Z",
  "finished_at": "2026-01-27T12:10:00Z",
  "exit_code": 0,
  "timed_out": false
}
```

Minimum `workspace.json` shape:

```json
{
  "schema_version": 1,
  "workspace_mode": "worktree",
  "base_sha": "abc123",
  "workspace_path": "/home/user/.tessariq/worktrees/example/01J...",
  "repo_mount_mode": "rw",
  "repo_clean": true,
  "reproducibility": "strong"
}
```

Minimum `index.jsonl` entry shape:

```json
{
  "run_id": "01J...",
  "created_at": "2026-01-27T12:00:00Z",
  "task_path": "specs/example.md",
  "task_title": "Example task",
  "adapter": "claude-code",
  "workspace_mode": "worktree",
  "state": "success",
  "evidence_path": ".tessariq/runs/01J..."
}
```

## Acceptance scenarios

- `init` creates the expected local skeleton and `.gitignore` entry
- `run` succeeds on a clean repo and creates the required evidence files
- `run` fails early on a dirty repo before container start
- `attach` works for a live run and fails cleanly for a finished run
- `promote` creates exactly one commit from a finished run with code changes
- `promote` creates no branch and no commit for a zero-diff run
- `promote` fails cleanly if required evidence is missing
- `proxy` mode enforces destination allowlists and records the compiled configuration
