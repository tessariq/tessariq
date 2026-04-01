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
- a minimal Tessariq reference runtime plus per-agent auth reuse is a workable local foundation

## Product intent

Tessariq v0.1.0 provides a Git-native, sandboxed way to run coding agents against a repository. The release is centered on five user-visible contracts:

- runs are detached by default, with optional attach to a live `tmux` session
- the default workspace is isolated from the repository working tree
- evidence is durable, repo-local, and stable enough for later automation
- promotion produces exactly one reviewable Git commit or fails cleanly
- a selected agent can reuse the user's existing supported auth state inside a compatible container runtime image without exposing host `HOME`

## Goals

1. Excellent local developer UX.
2. Stable run identity, lifecycle, and evidence layout.
3. Safe-by-default networking, with explicit unsafe opt-in for `open`.
4. Evidence contracts that later automation can parse without guessing.
5. A clear separation between user-facing agent selection and runtime-image execution details.

## Non-goals

- `copy+patch`
- `repo-rw`
- `resume`
- `inspect`, `logs`, `list`, `stop`, `clean`, `doctor`
- Kubernetes or distributed execution
- multi-agent orchestration
- web UI or database
- automatic push or PR creation
- tracking or pinning upstream third-party agent versions as a Tessariq product responsibility
- devcontainer-derived runtime support
- writable host credential or config mounts for agent auth refresh

## Host prerequisites

Tessariq v0.1.0 depends on a small set of host-side prerequisites.

- required host binaries:
  - `git` for repository discovery and Git operations
  - `tmux` for run session creation and attach flows
  - `docker` for containerized run execution
- prerequisite checks MUST fail before run-time side effects that depend on those prerequisites
- prerequisite failures MUST identify the missing or unavailable dependency and tell the user to install or enable it before retrying

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
- on Linux, the worktree directory MAY be made world-accessible before container start to allow the container's non-root user to write to bind-mounted paths; this is an accepted trade-off for single-user developer machines and is safe because worktrees are disposable, single-run directories

## CLI

### `tessariq init`

Creates the runtime state directory:

- `.tessariq/runs/`

It MUST NOT create `specs/` or any other user-managed directories.

It MUST add `.tessariq/` to `.gitignore`.

It MUST fail cleanly if `git` is unavailable.

### `tessariq version`

- prints `tessariq v<version>` to stdout
- supports both `tessariq version` and `tessariq --version`
- both invocation forms MUST produce identical output
- MUST work without repository context or generated Tessariq state
- MUST NOT require `git`, `tmux`, or `docker`

### `tessariq run <task-path>`

Detached by default.

The command MUST fail cleanly if required host prerequisites for run execution are unavailable (`git`, `tmux`, or `docker`).

Defaults:

- `--timeout=30m`
- `--grace=30s`
- `--agent=claude-code`
- `--egress=auto`
- `--interactive=false`
- `--attach=false`
- `--mount-agent-config=false`

Supported flags:

- `--agent claude-code|opencode`
- `--image <image>`
- `--model <string>`
- `--interactive`
- `--mount-agent-config`
- `--egress none|proxy|open|auto`
- `--unsafe-egress` as an alias for `--egress open`
- `--egress-allow <host[:port]>` repeatable
- `--egress-no-defaults`
- `--pre "<cmd>"` repeatable
- `--verify "<cmd>"` repeatable
- `--attach`

Hook execution context:

- `--pre` and `--verify` commands execute on the **host**, outside the container sandbox, with the invoking user's full privileges
- hooks are not subject to container isolation, egress restrictions, or capability limits
- users are responsible for reviewing and trusting hook commands before use
- hook output is captured in `runner.log`

Default tool-permission mode:

- by default the agent runs autonomously inside the container sandbox without requiring human approval for tool use
- `--interactive` opts in to human-in-the-loop approval; this is intended for use with `--attach` where a human is present to approve each tool invocation
- `--interactive` without `--attach` is valid but will cause the agent to block waiting for approval with no terminal attached

Runtime-image behavior:

- Tessariq MUST ship one official minimal reference runtime image for v0.1.0
- the reference runtime image for v0.1.0 MUST be published as `ghcr.io/tessariq/reference-runtime:v0.1.0`
- the reference runtime image MUST use a glibc-based Linux base image and a non-root default user
- the reference runtime image MUST support general JS/TS, Python, and Go development tooling
- the reference runtime image baseline MUST include at least `bash`, `ca-certificates`, `curl`, `git`, `jq`, `ripgrep`, `zip`, `unzip`, `tar`, `xz-utils`, `patch`, `procps`, `less`, `openssh-client`, `make`, `build-essential`, `pkg-config`, Python 3 with `pip` and `venv`, Node LTS with `npm` and `corepack`, and Go `1.26`
- the reference runtime image MUST NOT bundle third-party agent binaries such as Claude Code or OpenCode
- `--image` overrides the runtime image for that run
- the selected agent binary MUST already exist in the resolved runtime image

Container security posture:

- Tessariq MUST create agent containers with all Linux capabilities dropped (`--cap-drop=ALL`)
- Tessariq MUST prevent in-container privilege escalation (`--security-opt=no-new-privileges`)
- custom seccomp profiles and container resource limits (memory, CPU, PIDs) are out of scope for v0.1.0

Workspace repair containers:

- when Tessariq uses a helper container to repair worktree ownership after a run, the repair image MUST be pinned by digest rather than a mutable tag
- repair containers MUST only mount the disposable worktree path and MUST NOT broaden access to evidence, auth, or config mounts

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
- MUST fail cleanly if `tmux` is unavailable

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

## Agent and Runtime Contract

v0.1.0 supports exactly these first-party agents:

- `claude-code`
- `opencode`

Common agent rules:

- each run MUST write `agent.json`
- `agent.json` MUST record requested agent options
- if an option such as `--model` or `--interactive` cannot be applied exactly, the selected agent MUST record that it was requested but not applied

Common runtime rules:

- each run MUST write `runtime.json`
- Tessariq MUST treat `agent` as the user-facing tool choice and the runtime image as the execution environment
- v0.1.0 local auth reuse support is limited to Linux and macOS hosts
- Tessariq MUST NOT expose the host `HOME` directory inside the container
- for each supported agent, Tessariq MUST maintain documented knowledge of the host auth files or directories required for that agent to reuse existing local authentication
- for each supported agent, Tessariq MUST auto-detect the required auth files or directories and mount them read-only when present
- Claude Code required auth paths are:
  - Linux: `~/.claude/.credentials.json` and `~/.claude.json`
  - macOS: `~/.claude/.credentials.json` when a file-backed credential mirror is present, and `~/.claude.json`
- OpenCode required auth paths are:
  - Linux and macOS: `~/.local/share/opencode/auth.json`
- `--mount-agent-config` MUST opt in to additional read-only mounting of the supported agent's default config directories
- `--mount-agent-config` default config directories are:
  - Claude Code: `~/.claude/`
  - OpenCode: `~/.config/opencode/`
- required and optional mounts MUST use deterministic in-container destinations under the container user's home directory:
  - Claude Code: `$HOME/.claude/.credentials.json`, `$HOME/.claude.json`, and `$HOME/.claude/`
  - OpenCode: `$HOME/.local/share/opencode/auth.json` and `$HOME/.config/opencode/`
- when Claude Code config directories are mounted, Tessariq MUST set `CLAUDE_CONFIG_DIR=$HOME/.claude` inside the container
- Tessariq MUST NOT mount arbitrary host-home paths as a side effect of `--mount-agent-config`
- missing or unreadable optional agent config directories MUST warn and be recorded in run artifacts, but MUST NOT fail a run when required auth mounts are valid
- direct reuse of the macOS Keychain for Claude Code is out of scope for v0.1.0; Claude Code auth reuse on macOS is supported only when the file-backed credential mirror exists
- Tessariq MUST NOT support auth flows that require writable credential or config mounts in v0.1.0
- Tessariq MUST fail cleanly when the selected agent requires writable auth refresh behavior that is incompatible with the read-only mount contract
- the confidentiality of mounted auth credentials depends on egress enforcement restricting which hosts the agent can reach; read-only auth mounts and egress restrictions are complementary controls and neither is sufficient alone

## Adapter contract

Historical alias for completed planning tasks created before the v0.1.0 shift from adapter-centric wording to the agent/runtime model.

The current normative contract lives in `Agent and Runtime Contract` above.

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
- `allowlist_source` in `manifest.json` and `egress.compiled.yaml` MUST be one of `cli`, `user_config`, or `built_in`

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
- the built-in profile MUST also include the maintained HTTPS destinations required for the selected supported agent to authenticate and operate normally under the documented v0.1.0 contract
- the initial v0.1.0 baseline profile MUST include at least npm, PyPI, RubyGems, crates.io, the Go module proxy and checksum database, Maven Central, and Wikipedia over TCP `443`
- the implementation MUST maintain a documented per-agent endpoint profile for Claude Code and a documented provider-aware endpoint profile for OpenCode so `auto` remains predictable as endpoints evolve
- Claude Code built-in endpoints MUST include `api.anthropic.com:443`, `claude.ai:443`, and `platform.claude.com:443`
- OpenCode built-in endpoints MUST include `models.dev:443` and the resolved provider base-URL host on `443`
- `opencode.ai:443` MUST be added only when the resolved OpenCode configuration uses an OpenCode-hosted provider or auth flow that requires it
- when OpenCode is selected and Tessariq cannot determine the provider host required for `--egress auto` from the available config and auth state, Tessariq MUST fail before container start and tell the user to configure the provider explicitly or use `--egress-allow`
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
- `agent.json`
- `runtime.json`
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

### Evidence permissions

- evidence directories MUST be created with permissions `0o700` (owner-only access)
- evidence files MUST be created with permissions `0o600` (owner-only read/write)
- evidence is intended for the invoking user only and MUST NOT be world-readable

### Compatibility rules

- every emitted JSON artifact defined by this spec MUST include `schema_version`
- v0.1.0 defines `schema_version: 1` for `manifest.json`, `status.json`, `agent.json`, `runtime.json`, and `workspace.json`
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
  "agent": "claude-code",
  "base_sha": "abc123",
  "workspace_mode": "worktree",
  "requested_egress_mode": "auto",
  "resolved_egress_mode": "proxy",
  "allowlist_source": "built_in",
  "container_name": "tessariq-01J...",
  "created_at": "2026-01-27T12:00:00Z"
}
```

Minimum `agent.json` shape:

```json
{
  "schema_version": 1,
  "agent": "claude-code",
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

Minimum `runtime.json` shape:

```json
{
  "schema_version": 1,
  "image": "ghcr.io/tessariq/reference-runtime:v0.1.0",
  "image_source": "reference",
  "auth_mount_mode": "read-only",
  "agent_config_mount": "disabled",
  "agent_config_mount_status": "disabled"
}
```

Required proxy-mode `egress.compiled.yaml` fields:

```yaml
schema_version: 1
allowlist_source: built_in
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
  "agent": "claude-code",
  "workspace_mode": "worktree",
  "state": "success",
  "evidence_path": ".tessariq/runs/01J..."
}
```

## Acceptance scenarios

- `init` creates `.tessariq/runs/` and the `.gitignore` entry
- `run` succeeds on a clean repo with a compatible runtime image and creates the required evidence files
- `run` fails early on a dirty repo before container start
- `run` fails early with actionable guidance when a required host prerequisite is missing or unavailable
- `run` auto-detects supported agent auth state and mounts it read-only when present
- `run` reuses Claude Code auth on macOS only when a file-backed `~/.claude/.credentials.json` credential mirror is present
- `run --mount-agent-config` additionally mounts the selected supported agent's default config directories read-only
- `run --mount-agent-config` warns and records mount status when optional config directories are missing or unreadable, but continues when required auth mounts are valid
- `run` does not expose the host `HOME` directory inside the container
- `run` fails cleanly when the selected agent binary is missing from the resolved runtime image
- `run` fails cleanly when required supported agent auth state is missing
- `run` fails cleanly when the selected agent requires writable auth refresh behavior
- `version` succeeds without repository context and prints the expected version line
- `init` fails cleanly with actionable guidance when `git` is unavailable
- `attach` works for a live run and fails cleanly for a finished run
- `attach` fails cleanly with actionable guidance when `tmux` is unavailable
- `promote` creates exactly one commit from a finished run with code changes
- `promote` creates no branch and no commit for a zero-diff run
- `promote` fails cleanly if required evidence is missing
- `proxy` mode enforces destination allowlists and records the compiled configuration
- `auto` uses the built-in allowlist profile when no explicit allowlist source is present
- `auto` includes the maintained endpoints required for Claude Code and the resolved provider endpoints required for OpenCode
- `run --agent opencode --egress auto` fails before container start when the required provider host cannot be determined from the available config and auth state
- user-level config replaces the built-in allowlist profile for `auto`
- CLI `--egress-allow` values override user-level config and the built-in allowlist profile

## Failure UX

| Condition | Required behavior | Required user guidance |
| --- | --- | --- |
| task path is missing, outside the repository, or not Markdown | fail before container start | print the invalid path and tell the user to pass a Markdown task file inside the current repository |
| repository is dirty for `worktree` | fail before container start | tell the user to commit, stash, or clean the repository first |
| required host prerequisite (`git`, `tmux`, or `docker`) is missing or unavailable | fail before dependent command work begins | identify which prerequisite is missing or unavailable and tell the user to install or enable it, then retry |
| the selected agent binary is missing from the resolved runtime image | fail before agent start | identify the missing binary, name the selected agent, and tell the user to use a compatible runtime image or `--image` override |
| required supported agent auth state is missing | fail before agent start | identify that supported auth files or directories for the selected agent were not found and tell the user to authenticate that agent locally first |
| Claude Code is selected on macOS and only Keychain-backed auth exists with no file-backed credential mirror | fail before agent start | explain that v0.1.0 supports Claude Code auth reuse on macOS only when `~/.claude/.credentials.json` exists and tell the user to use a compatible file-backed setup |
| the selected agent requires writable auth refresh or config mutation | fail before agent start | explain that v0.1.0 supports only read-only auth and config mounts and tell the user to use a compatible pre-authenticated setup |
| `--mount-agent-config` is enabled and optional config directories are missing or unreadable | continue the run if required auth mounts are valid, and record a warning in run artifacts | tell the user which optional config path could not be mounted and that Tessariq continued with required auth mounts only |
| OpenCode is selected with `--egress auto` and the provider host cannot be determined | fail before container start | tell the user to configure the provider explicitly so Tessariq can derive the required host, or pass `--egress-allow` manually |
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

### Proxy mode runtime sketch

The current implementation direction for `proxy` is:

- create a per-run internal `run_net`
- start a per-run Squid proxy container connected to `run_net` and a non-internal egress network
- run the agent container only on `run_net`
- configure `HTTP_PROXY` and `HTTPS_PROXY` for the agent

### Reference runtime baseline

The current reference-runtime direction for `ghcr.io/tessariq/reference-runtime:v0.1.0` is:

- `debian:bookworm-slim` or an equivalent glibc-based base image
- a non-root default user named `tessariq`
- the baseline toolchain listed in the normative runtime-image contract above
- no bundled third-party agent binaries

### Supported auth and config paths

Current supported auth and config reuse paths are:

- Claude Code:
  - required auth:
    - Linux: `~/.claude/.credentials.json`
    - macOS: `~/.claude/.credentials.json` when a file-backed credential mirror exists
    - Linux and macOS: `~/.claude.json`
  - optional config via `--mount-agent-config`:
    - `~/.claude/`
- OpenCode:
  - required auth:
    - Linux and macOS: `~/.local/share/opencode/auth.json`
  - optional config via `--mount-agent-config`:
    - `~/.config/opencode/`

### Future macOS Claude helper sketch

Direct Claude Code Keychain reuse is not part of the v0.1.0 contract. A future host-helper approach on macOS could look like this:

```sh
#!/bin/sh
set -eu

tmp="$(mktemp)"
chmod 600 "$tmp"

security find-generic-password -a "$USER" -s "Claude Code-credentials" -w > "$tmp"

printf '%s\n' "$tmp"
```

The intended future flow is:

- run the helper on the macOS host
- write a short-lived temp credentials file with mode `0600`
- mount that file read-only into the container at `$HOME/.claude/.credentials.json`
- delete the temp file after the run completes

This helper sketch is informative only. It is not a supported v0.1.0 auth path and must not use `CLAUDE_CODE_OAUTH_TOKEN` because of the known upstream macOS side effects around Keychain state.

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

## Specification changelog

### 2026-04-01: Add version command contract to v0.1.0

**Changed:**

1. **v0.1.0 now includes a small version-reporting command**
   - `tessariq version` is part of the normative CLI surface.
   - The root command also supports `tessariq --version`.
   - Both invocation forms print the same `tessariq v<version>` line.
   - Rationale: version reporting is a basic CLI capability that should not require repository context or run-state setup.

### 2026-03-31: Security hardening amendments

**Changed:**

1. **Hook execution context is now explicitly documented**
   - `--pre` and `--verify` commands execute on the host, outside the container sandbox, with the invoking user's full privileges.
   - Rationale: the spec's security model centers on the container as the safety boundary. Hooks bypass that boundary entirely, and users need to understand this trust distinction.

2. **Container security posture now requires capability dropping and privilege escalation prevention**
   - Agent containers MUST be created with `--cap-drop=ALL` and `--security-opt=no-new-privileges`.
   - Custom seccomp profiles and resource limits (memory, CPU, PIDs) are explicitly deferred to a future release.
   - Rationale: a non-root user alone is insufficient container hardening for a product whose core value prop is sandboxed execution.

3. **Workspace repair containers MUST use pinned images**
   - Helper containers used for worktree ownership repair MUST use images pinned by digest, not mutable tags like `:latest`.
   - Rationale: a compromised `:latest` tag running as root with a worktree mount is a supply chain risk.

4. **Evidence files MUST be owner-only accessible**
   - Evidence directories: `0o700`. Evidence files: `0o600`.
   - Rationale: evidence contains task content, agent output, and configuration details that should not be readable by other users on the system.

5. **Credential confidentiality depends on egress enforcement**
   - The spec now explicitly states that read-only auth mounts and egress restrictions are complementary controls, and neither is sufficient alone.
   - Rationale: a mounted credential file is exfiltrable if the agent has unrestricted network access.

6. **Worktree permission window on Linux is documented as an accepted trade-off**
   - On Linux, worktrees may be made world-accessible before container start to allow the non-root container user to write to bind-mounted paths.
   - This is acceptable because worktrees are disposable, single-run directories on single-user developer machines.

**Tasks affected:**

- New task TASK-030 for timeout signal escalation fix (SIGTERM before SIGKILL, per TASK-005 acceptance criteria).
- New task TASK-031 for pinning the workspace repair container image by digest.
- New task TASK-032 for container security hardening (`--cap-drop=ALL`, `--security-opt=no-new-privileges`, evidence file permissions).

### 2026-03-30: Shift v0.1.0 to the agent and runtime model

**Changed:**

1. **User-facing terminology now uses `agent` instead of `adapter`**
   - `adapter` was an internal implementation term that leaked unnecessary runtime details.
   - v0.1.0 now treats `agent` as the user-facing concept in CLI semantics, evidence, and planning references.

2. **The evidence contract now splits agent metadata from runtime metadata**
   - Old: `adapter.json` mixed requested/applied agent options with runtime image identity.
   - New: `agent.json` records requested/applied agent options, and `runtime.json` records runtime-image and mount-policy metadata.
   - `manifest.json` and `index.jsonl` now use `agent` instead of `adapter`.

3. **v0.1.0 now defines one official minimal Tessariq reference runtime image**
   - The reference runtime supports JS/TS, Python, and Go development tooling.
   - It intentionally does not bundle third-party agent binaries.
   - Rationale: Tessariq should own a safe, minimal runtime foundation without becoming a third-party agent version manager in v0.1.0.

4. **The selected agent must already exist in the resolved runtime image**
   - `--image` remains the runtime-image override.
   - Tessariq reuses supported auth state inside a compatible image; it does not reuse the host-installed binary.

5. **Auth and config mounts are now explicit parts of the v0.1.0 contract**
    - Tessariq MUST auto-detect and mount required supported-agent auth paths read-only.
    - `--mount-agent-config` opts in to read-only mounting of supported default agent config directories.
    - Tessariq MUST NOT expose host `HOME` and MUST NOT support writable auth refresh flows in v0.1.0.

6. **Auth reuse is now concretely defined for Linux and macOS hosts**
   - Claude Code uses `~/.claude/.credentials.json` and `~/.claude.json` on Linux.
   - Claude Code on macOS is supported only when a file-backed `~/.claude/.credentials.json` credential mirror exists; direct Keychain reuse is out of scope for v0.1.0.
   - OpenCode uses `~/.local/share/opencode/auth.json` for required auth and `~/.config/opencode/` as the optional config-dir mount.

7. **The reference runtime image now has a concrete baseline contract**
   - The v0.1.0 image is pinned to `ghcr.io/tessariq/reference-runtime:v0.1.0`.
   - It uses a glibc-based Linux base image, a non-root user, and a defined baseline toolchain for JS/TS, Python, and Go work.

8. **`auto` egress is now agent-aware for Claude Code and provider-aware for OpenCode**
   - The built-in allowlist profile still covers common package-manager workflows.
   - It now also includes the maintained endpoints required for Claude Code and the resolved provider endpoints required for OpenCode.

9. **Optional config-dir mounts now have warning-only behavior**
   - Missing or unreadable optional config directories do not fail a run when required auth mounts are valid.
   - Tessariq must warn the user and record the outcome in run artifacts and `runtime.json`.

10. **A future macOS host-helper pattern is documented for Claude Code Keychain export**
   - The spec now includes an informative helper sketch that exports a Keychain credential to a short-lived temp file for future container mounting.
   - This is documented as a future approach only, not as part of the supported v0.1.0 contract.

**Tasks affected:**

- Existing done tasks that reference `adapter` terminology or `adapter.json` are now historical and need supersession notes plus updated spec anchors.
- Existing open tasks for egress, evidence, run index, verification, and closeout must be rewritten around `agent.json`, `runtime.json`, auth/config mounts, and agent-aware `auto` egress.
- New tasks are required for the official reference runtime image, supported-agent auth reuse, `--mount-agent-config`, and later runtime baking work.

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

4. **Agent-option evidence example updated**
   - The minimum requested/applied option shape now uses `interactive` instead of `yolo`.

**Tasks affected:** TASK-002 (done, code update tracked in TASK-018), TASK-009, TASK-010, TASK-011. New tasks created: TASK-018 (v0.1.0, code changes), TASK-019 (v0.2.0, `--prompt` backlog).
