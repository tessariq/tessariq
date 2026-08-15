# Tessariq Command Reference

Deep-dive reference for command behavior that needs more than a flag list. The
[README](../README.md) covers installation and the core `run -> attach if needed
-> promote` workflow. Run `tessariq <command> --help` for the complete flag list.

The versioned [product specs](https://github.com/tessariq/tessariq/blob/main/specs/README.md)
remain the normative source of truth when this guide and a spec disagree.

## Initialize repository state

`tessariq init` creates the repo-local `.tessariq/` runtime area used for run
indexes and evidence. Runtime state is excluded from Git and must never be
committed.

```sh
tessariq init
```

Initialization is safe to run again in an already initialized repository.

## Run a task

`tessariq run <task-path>` executes a Markdown task from the current Git
repository in an isolated Docker container and detached Git worktree.

```sh
tessariq run tasks/fix-login.md
```

The task path must resolve to a Markdown file inside the current repository.
Before allocating a run, Tessariq refuses repositories with staged, unstaged, or
untracked non-ignored changes. This protects the recorded base commit and keeps
the active worktree out of the agent's write path.

The selected agent defaults to its published quickstart image. Use `--image` to
supply a compatible image; see [Runtime Images](runtime-images.md). Supported
credentials and optional agent configuration are mounted read-only. The host
home directory is not exposed to the container.

Egress defaults to `auto` and can be selected explicitly with `--egress none`,
`proxy`, or `open`. Proxy allowlist options apply only to proxy mode; incompatible
combinations are rejected rather than silently ignored.

## Run output

After the agent and verification hooks finish successfully, `run` prints stable,
script-friendly fields:

```text
run_id: 01ARZ3NDEKTSV4RRFFQ69G5FAV
evidence_path: /path/to/repo/.tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV
workspace_path: /home/user/.tessariq/worktrees/repo-12345678/01ARZ3NDEKTSV4RRFFQ69G5FAV
container_name: tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV
attach: tessariq attach 01ARZ3NDEKTSV4RRFFQ69G5FAV
promote: tessariq promote 01ARZ3NDEKTSV4RRFFQ69G5FAV
```

Terminal non-success states after allocation print `run_id`, `state`, and
`evidence_path`, plus a cleanup error when one occurred. Other post-bootstrap
failures print at least `run_id` and `evidence_path`. They omit fields that would
claim a usable workspace, container, attach session, or promotable result.
Failures before allocation, including a dirty repository or invalid task path,
do not print a run ID.

## Run references

Commands that accept `<run-ref>` accept a run ID, `last`, or `last-N`. Resolution
uses the current repository's index: `last` and `last-0` select its newest unique
run, `last-1` the previous unique run, and so on.

```sh
tessariq attach last
tessariq promote last
```

Resolution does not skip runs that are unsuitable for the requested command.
For example, `attach last` fails when the newest run is not live rather than
searching backward for a live run. Use an explicit run ID in scripts when the
selection must not depend on later activity.

## Attach to a run

`tessariq attach <run-ref>` joins a live run's `tmux` session. Detach without
stopping the run by pressing `Ctrl-b d`.

```sh
tessariq attach 01ARZ3NDEKTSV4RRFFQ69G5FAV
```

`run --attach` starts a run and immediately joins its session. Interactive agent
interfaces require the command's interactive mode and a terminal.

## Promote a run

`tessariq promote <run-ref>` validates the completed run and creates exactly one
branch and one commit from its recorded base and patch. It does not commit from
the retained execution worktree.

```sh
tessariq promote last
```

The branch is named `tessariq/<run_id>`. The commit records `Tessariq-Run`,
`Tessariq-Base`, and `Tessariq-Task` trailers. Promotion fails before creating a
branch or commit when required evidence is absent, malformed, or inconsistent,
or when the run produced no code changes.

Promotion checks evidence consistency; it is not an integrity boundary against
an actor who can coherently rewrite the entire `.tessariq/runs/` tree.

## Evidence left by a run

Every allocated run writes owner-only evidence beneath
`.tessariq/runs/<run_id>/`. Required and conditional artifacts include:

```text
.tessariq/
  runs/
    index.jsonl
    <run_id>/
      manifest.json        # task identity and resolved execution settings
      status.json          # lifecycle state, timing, and exit code
      agent.json           # requested and supported agent options
      runtime.json         # image identity and mount policy
      workspace.json       # workspace path, base SHA, and reproduction data
      task.md              # exact task supplied to the agent
      diff.patch           # emitted when code changed
      diffstat.txt         # emitted when code changed
      run.log              # captured agent output
      runner.log           # host runner and hook output
      egress.compiled.yaml # emitted in proxy mode
      egress.events.jsonl  # emitted in proxy mode
      timeout.flag         # emitted when timeout escalation begins
```

Structured evidence remains valid on terminal run failures. Files are plain
JSON, YAML, Markdown, and text so they can be inspected without Tessariq or a
database.

The host-side runner owns and writes evidence. The agent can write only inside
the worktree mounted at `/work`; the evidence mount at `/evidence`, auth state,
and optional agent configuration are read-only from the container.

## Isolation and cleanup

Each run gets a deterministic container and a detached worktree under
`~/.tessariq/worktrees/<repo-id>/<run-id>`. The agent container runs as the named
non-root `tessariq` user with Linux capabilities dropped and privilege escalation
disabled.

Successful run worktrees are retained for inspection. Failed, interrupted, and
timed-out runs remove their disposable worktrees during cleanup. Promotion uses
recorded evidence rather than depending on a retained execution worktree.

Docker bind-mount UID behavior differs by platform: Linux preserves host UIDs,
while Docker Desktop on macOS maps them through its filesystem layer. Tessariq
prepares disposable worktrees so the isolated container user can write and so
the host can remove them afterwards.

## Networking and egress

Proxy mode creates a per-run Docker network and Squid proxy. Allowlists are
enforced at `host:port` granularity, with port 443 used when none is specified.
The compiled destinations and blocked attempts are recorded in the run evidence
for auditability.

Allowlist sources use replacement precedence, not additive merging. One or more
`--egress-allow` values produce exactly the CLI destinations; otherwise user
configuration replaces the built-in profile; otherwise the built-in profile is
used. `--egress-no-defaults` discards configured and built-in destinations, so
proxy mode then requires CLI destinations. The selected source is written into
`manifest.json` and `egress.compiled.yaml`.

Open mode deliberately bypasses proxy enforcement. None mode gives the agent no
external network path. Consult the active spec for the complete normative egress
contract and security tradeoffs.
