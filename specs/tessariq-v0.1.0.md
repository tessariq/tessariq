# Tessariq v0.1.0 Specification

**Status:** In Progress  
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

- runtime state is generated under `<repo>/.tessariq/` at the repository root, as a sibling of `specs/`
- `tessariq init` MUST add `.tessariq/` to `.gitignore`
- repo-tracked config files are out of scope for v0.1.0
- v0.1.0 MAY read user-level defaults from `$XDG_CONFIG_HOME/tessariq/config.yaml` or `~/.config/tessariq/config.yaml` when the XDG location is unset
- the only normative user-level config surface in v0.1.0 is default proxy allowlist selection for `--egress=auto`
- CLI flags remain the per-run source of truth and override user-level defaults

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

Creates the runtime state directory:

- `.tessariq/runs/`

It MUST NOT create `specs/` or any other user-managed directories.

It MUST add `.tessariq/` to `.gitignore`.

### `tessariq run <task-path>`

Detached by default.

Defaults:

- `--timeout=30m`
- `--grace=30s`
- `--agent=claude-code`
- `--egress=auto`
- `--interactive=false`
- `--attach=false`

Supported flags:

- `--agent claude-code|opencode`
- `--image <image>`
- `--model <string>`
- `--interactive`
- `--egress none|proxy|open|auto`
- `--unsafe-egress` as an alias for `--egress open`
- `--egress-allow <host[:port]>` repeatable
- `--egress-no-defaults`
- `--pre "<cmd>"` repeatable
- `--verify "<cmd>"` repeatable
- `--attach`

Default tool-permission mode:

- by default the agent runs autonomously inside the container sandbox without requiring human approval for tool use
- `--interactive` opts in to human-in-the-loop approval; this is intended for use with `--attach` where a human is present to approve each tool invocation
- `--interactive` without `--attach` is valid but will cause the agent to block waiting for approval with no terminal attached

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
- if an option such as `--model` or `--interactive` cannot be applied exactly, the adapter MUST record that it was requested but not applied

Minimum `adapter.json` shape:

```json
{
  "schema_version": 1,
  "adapter": "claude-code",
  "image": "example/image:tag",
  "requested": {
    "model": "gpt-5.4",
    "interactive": true
  },
  "applied": {
    "model": false,
    "interactive": true
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
- when no explicit allowlist is provided by user config or CLI flags, `auto` MUST use the built-in Tessariq allowlist profile

User-level config:

- user-level config MAY define a replacement default allowlist for `proxy` and `auto`
- repo-tracked project config remains out of scope for v0.1.0
- CLI `--egress-allow` entries MUST override user-level config for that run
- `--egress-no-defaults` MUST discard any built-in or user-configured default allowlist before later `--egress-allow` entries are applied

Allowlist precedence:

- if one or more CLI `--egress-allow` values are provided, the resolved allowlist MUST contain exactly those CLI destinations
- otherwise, if user-level config defines a default allowlist, the resolved allowlist MUST contain exactly the configured destinations
- otherwise, the resolved allowlist MUST contain the built-in Tessariq allowlist profile

Built-in Tessariq allowlist profile:

- the built-in profile MUST include maintained HTTPS destinations for common package-manager workflows and Wikipedia
- the initial v0.1.0 profile MUST include at least npm, PyPI, RubyGems, crates.io, the Go module proxy and checksum database, Maven Central, and Wikipedia over TCP `443`
- the fully resolved allowlist MUST be written to `egress.compiled.yaml`

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
- proxy-mode evidence MUST record both allowlist provenance and the fully resolved destinations without requiring the caller to re-derive them

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
  "allowlist_source": "auto",
  "container_name": "tessariq-01J...",
  "created_at": "2026-01-27T12:00:00Z"
}
```

Required proxy-mode `egress.compiled.yaml` fields:

```yaml
schema_version: 1
allowlist_source: auto
destinations:
  - host: registry.npmjs.org
    port: 443
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

- `init` creates `.tessariq/runs/` and the `.gitignore` entry
- `run` succeeds on a clean repo and creates the required evidence files
- `run` fails early on a dirty repo before container start
- `attach` works for a live run and fails cleanly for a finished run
- `promote` creates exactly one commit from a finished run with code changes
- `promote` creates no branch and no commit for a zero-diff run
- `promote` fails cleanly if required evidence is missing
- `proxy` mode enforces destination allowlists and records the compiled configuration
- `auto` uses the built-in allowlist profile when no explicit allowlist source is present
- user-level config replaces the built-in allowlist profile for `auto`
- CLI `--egress-allow` values override user-level config and the built-in allowlist profile

## Failure UX

| Condition | Required behavior | Required user guidance |
| --- | --- | --- |
| task path is missing, outside the repository, or not Markdown | fail before container start | print the invalid path and tell the user to pass a Markdown task file inside the current repository |
| repository is dirty for `worktree` | fail before container start | tell the user to commit, stash, or clean the repository first |
| proxy mode blocks a destination that is not present in the resolved allowlist | fail the network attempt and record it in proxy evidence | tell the user which `host:port` was blocked and how to add it through user config or CLI flags, or to rerun with explicit open egress |
| `attach` references an unknown or finished run | fail without attaching | print the evidence path when known and tell the user the run is not live |
| `promote` sees zero diff | fail without creating a branch or commit | tell the user there were no code changes to promote |
| `promote` cannot find required evidence | fail without creating a branch or commit | identify the missing artifact and tell the user the run cannot be promoted until evidence is intact |

## Success metrics

- at least 90% of started runs end with the required evidence set present and parseable
- at least 70% of finished runs with code changes are promotable without manual evidence repair
- at least 80% of proxy-mode runs succeed without requiring `--egress open`
- fewer than 15% of proxy-mode failures are caused by missing allowlist destinations after one corrective rerun
- fewer than 40% of successful runs require `attach`, preserving detached-by-default as the normal path

## Implementation Notes (Informative)

This section is informative. It describes the current implementation shape for v0.1.0, and the normative sections above take precedence if there is any conflict.

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

### Proxy mode runtime sketch

The current implementation direction for `proxy` is:

- create a per-run internal `run_net`
- start a per-run Squid proxy container connected to `run_net` and a non-internal egress network
- run the agent container only on `run_net`
- configure `HTTP_PROXY` and `HTTPS_PROXY` for the agent

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

## Specification changelog

### 2026-03-29: Replace `--yolo` with `--interactive`, rename `--egress-allow-reset`

**Changes:**

1. **`--yolo` replaced by `--interactive` (inverted default)**
   - Old: `--yolo` (default `false`) opted in to autonomous tool use
   - New: `--interactive` (default `false`) opts in to human-in-the-loop tool approval
   - Default behavior is now autonomous: the agent runs tools without human approval inside the container sandbox
   - Rationale: tessariq's core workflow is detached-by-default. A detached agent with no human terminal cannot receive tool-approval prompts and would hang until timeout. The container is the safety boundary; requiring per-tool approval inside an isolated container with controlled egress is redundant. `--interactive` is reserved for `--attach` workflows where a human is present.

2. **`--egress-allow-reset` renamed to `--egress-no-defaults`**
   - Old: `--egress-allow-reset` with description "discard built-in and user-configured allowlist"
   - New: `--egress-no-defaults` with description "ignore default allowlists; only --egress-allow entries apply"
   - Rationale: the old name implied destructive mutation ("reset"). The new name describes the declarative intent: do not include any built-in or user-configured defaults in the resolved allowlist. This also clarifies the distinction from `--egress none` (which disables all network access).

3. **`--interactive=false` added to documented defaults**
   - The defaults section now explicitly lists `--interactive=false` alongside `--timeout=30m`, `--grace=30s`, etc.
   - Rationale: making the autonomous default explicit in the spec removes ambiguity for implementers and users.

4. **`adapter.json` example updated**
   - The minimum `adapter.json` shape now uses `interactive` instead of `yolo` in the `requested` and `applied` fields.

**Tasks affected:** TASK-002 (done, code update tracked in TASK-018), TASK-009, TASK-010 (updated `--yolo` references to `--interactive`), TASK-011 (updated `--egress-allow-reset` references to `--egress-no-defaults`). New tasks created: TASK-018 (v0.1.0, code changes), TASK-019 (v0.2.0, `--prompt` backlog).
