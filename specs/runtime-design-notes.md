# Tessariq Runtime Design Notes

**Status:** Informative  
**Scope:** Implementation notes that support the release specs

This document is not the normative release contract. It collects lower-level implementation detail that may change without changing product intent, as long as the versioned specs remain satisfied.

## Purpose

Keep the release specs focused on:

- user-visible behavior
- release learning goals
- stable guarantees
- evidence compatibility

Keep runtime-heavy design detail here:

- generated storage layout
- derived local identifiers
- container and proxy topology
- runner and bootstrap responsibilities
- implementation hints for diff and log generation

## Current runtime sketch

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

### Proxy mode runtime sketch

The current implementation direction for `proxy` is:

- create a per-run internal `run_net`
- start a per-run Squid proxy container connected to `run_net` and a non-internal egress network
- run the agent container only on `run_net`
- configure `HTTP_PROXY` and `HTTPS_PROXY` for the agent

These details are informative until they become part of a user-visible compatibility requirement.

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

## Guidance

- If a detail changes user-visible guarantees, move it into the relevant versioned spec.
- If a detail is primarily about implementation strategy, keep it here.
- If a future automation consumer depends on a field or file shape, specify that contract in the versioned spec rather than only here.
