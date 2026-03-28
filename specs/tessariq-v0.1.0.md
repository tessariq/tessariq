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

Tessariq v0.1.0 provides a Git-native, sandboxed, reproducible way to run coding agents against a repository. It prioritizes:

- detached-by-default runs
- optional attach to a `tmux` session inside the container
- a two-phase workflow:
  - `run`: create workspace, execute the task, persist evidence
  - `promote`: create exactly one commit suitable for further review or PR work
- evidence-first execution: every run emits a durable evidence set

## Goals

1. Excellent local developer UX.
2. Deterministic run identity, artifact layout, and lifecycle.
3. Controlled network egress with explicit unsafe opt-in for `open`.
4. Evidence contracts that are stable enough for later automation.

## Non-goals

- `copy+patch`
- `repo-rw`
- `resume`
- `inspect`, `logs`, `list`, `stop`, `clean`, `doctor`
- Kubernetes or distributed execution
- multi-agent orchestration
- web UI or database
- automatic push or PR creation

## Core concepts

### Run

A run is identified by a ULID `run_id` and produces:

- a detached worktree workspace
- evidence under `<repo>/.tessariq/runs/<run_id>/`
- a container lifecycle
- a final state: `success|failed|timeout|killed|interrupted`

### Promote

`promote` converts a run output into:

- a branch
- exactly one commit containing code changes
- default commit trailers linking the commit back to the run

## Repository model

### User-authored files

User-authored task/spec Markdown is repo-tracked and lives at explicit paths inside the repository. Examples:

- `specs/fix-timeout-handling.md`
- `specs/release/tessariq-v0.1.0.md`

`tessariq run <task-path>` MUST accept a path inside the current repository.

### Generated runtime state

Runtime state is generated under `<repo>/.tessariq/` and is not user-authored product input.

`tessariq init` MUST add `.tessariq/` to `.gitignore`.

### Storage layout

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
        egress.compiled.yaml      # only in proxy mode
        egress.events.jsonl       # only in proxy mode
        squid.log                 # optional, capped
        timeout.flag              # internal marker
        bootstrap.sh              # generated
        runner.sh                 # generated
        workspace.json
```

```text
~/.tessariq/
  worktrees/
    <repo_id>/
      <run_id>/
```

Repo-local config files are out of scope for v0.1.0. Behavior is driven by CLI flags.

## Identifiers and run selection

### `run_id`

- MUST be a ULID.

### `repo_id`

- `repo_root = realpath(git rev-parse --show-toplevel)`
- `repo_id = slug(basename(repo_root)) + "-" + shortHash(repo_root)`
- `shortHash` is the first 8 hex chars of `sha256(repo_root)`

### Run references

Commands that accept `<run-ref>` MUST support:

- explicit `run_id`
- `last`
- `last-N`

Resolution order:

1. Resolve against `<repo>/.tessariq/runs/index.jsonl`.
2. Error if the run cannot be found.

## Prerequisites

### Host

- `git`
- `docker`
- permission to create containers and networks

### Agent image

The agent image MUST include:

- `/bin/sh`
- `tmux`

It SHOULD include:

- `date`
- `sleep`
- `kill`

## Task input contract

- `tessariq run <task-path>` requires a Markdown file inside the current repository.
- Tessariq MUST copy the exact task file into evidence as `task.md`.
- The first Markdown H1, if present, becomes `task_title`.
- If no H1 exists, `task_title` falls back to the task file basename without extension.

## Workspace model

v0.1.0 supports exactly one workspace mode: `worktree`.

### `worktree`

- Tessariq creates a detached worktree at `~/.tessariq/worktrees/<repo_id>/<run_id>`.
- The container mounts that worktree read-write at `/work`.
- Evidence is mounted separately at `/tessariq-run`.
- `workspace.json` MUST record:
  - `workspace_mode = "worktree"`
  - `base_sha`
  - `worktree_path`
  - `repo_mount_mode = "rw"`
  - `repo_clean = true`

## Base state policy

- `base_sha` is the current `HEAD` commit of the repository at run start.
- Tessariq MUST refuse to start a run if the repository has staged, unstaged, or untracked non-ignored files.
- The failure MUST happen before creating the container.
- The error message MUST tell the user to commit, stash, or clean the repository first.

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
- worktree path
- evidence path
- container name or id
- attach command
- promote command

### `tessariq attach <run-ref>`

- Attaches the user terminal to the run's `tmux` session.
- It MUST fail cleanly if the run is not live.
- The error MUST include the evidence path.

### `tessariq promote <run-ref>`

- MUST create a branch.
- MUST create exactly one commit.
- MUST use `git add -A`.
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

## Adapter contract

v0.1.0 supports exactly these first-party adapters:

- `claude-code`
- `opencode`

Common rules:

- each adapter MUST write `adapter.json`
- `adapter.json` MUST record the requested adapter options
- if an option such as `--model` or `--yolo` cannot be applied exactly, the adapter MUST record that it was requested but not applied

Minimum `adapter.json` shape:

```json
{
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

### `proxy`

The CLI MUST:

- create a per-run internal `run_net`
- start a per-run Squid proxy container connected to `run_net` and a non-internal egress network
- run the agent container only on `run_net`
- configure `HTTP_PROXY` and `HTTPS_PROXY`

Artifacts in proxy mode:

- `egress.compiled.yaml`
- `egress.events.jsonl`
- optional `squid.log`

Rules:

- allowlists are enforced at destination `host:port`
- default port is TCP `443`
- HTTPS and WSS use CONNECT tunneling
- URL-path filtering is out of scope

### `open`

- MUST require explicit opt-in
- MUST be recorded in `manifest.json`

## Evidence contract

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

Logs MUST be capped and MUST include a truncation marker if truncated.

Minimum `manifest.json` shape:

```json
{
  "run_id": "01J...",
  "repo_id": "tessariq-1234abcd",
  "repo_root": "/abs/path/to/repo",
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
  "workspace_mode": "worktree",
  "base_sha": "abc123",
  "worktree_path": "/home/user/.tessariq/worktrees/tessariq-1234abcd/01J...",
  "repo_mount_mode": "rw",
  "repo_clean": true
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

## Runtime model

### Runner

Runner, as PID1, MUST:

- start the `tmux` session
- enforce timeout
- write `timeout.flag` before termination escalation on timeout
- ensure `status.json` exists even if bootstrap fails
- write `runner.log`

### Bootstrap

Bootstrap MUST:

- run `pre` commands
- run the selected adapter
- run `verify` commands
- trap `EXIT`
- generate diff artifacts best-effort
- write the final `status.json`

## Acceptance criteria

- `init` creates the expected local skeleton and `.gitignore` entry.
- `run` succeeds on a clean repo and creates the required evidence files.
- `run` fails early on a dirty repo before container start.
- `attach` works for a live run and fails cleanly for a finished run.
- `promote` creates exactly one commit from a changed run.
- `promote` creates no branch and no commit for a zero-diff run.
- `proxy` mode enforces destination allowlists and records the compiled configuration.
