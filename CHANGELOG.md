# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added `tessariq run --attach` to attach the invoking terminal to the newly created tmux session for the run, providing live visibility into the agent's progress without a separate `tessariq attach` step.
- Added `tessariq version` and root `tessariq --version`, both printing the same single-line build version without requiring repository context.
- Added `tessariq init` repository bootstrap that creates `.tessariq/runs/` and idempotently updates `.gitignore` to keep runtime state untracked.
- Added `tessariq run <task-path>` CLI wiring with initial flag surface, task-path validation, and manifest bootstrap with stable preflight fields.
- Added pre-run clean-repository gating and task ingestion so runs fail early on dirty repos and record copied task context (`task.md`) plus derived task title metadata.
- Added detached worktree provisioning under `~/.tessariq/worktrees/<repo_id>/<run_id>` and `workspace.json` evidence with reproducibility-focused metadata.
- Added runner lifecycle evidence contracts, including durable `status.json`, `run.log`, `runner.log`, deterministic container naming, and timeout bookkeeping.
- Added detached-by-default tmux session startup with script-friendly stdout guidance for `attach` and `promote` commands.
- Added shared adapter evidence contract with `adapter.json` requested-versus-supported recording semantics for exact and partial option support.
- Added first-party `claude-code` adapter support integrated into run lifecycle execution and evidence output.
- Added first-party `opencode` adapter with partial-application recording for unsupported `--interactive` option and full `--model` forwarding.
- Added actionable binary-not-found error messages for both `claude-code` and `opencode` adapters naming the missing binary and container image expectation.
- Added per-agent auth discovery that auto-detects required auth files before agent start and fails with actionable messages when auth is missing, Keychain-only on macOS, or writable refresh is required.
- Added `--mount-agent-config` flag to `tessariq run` for opt-in read-only mounting of supported agents' default config directories (Claude Code `~/.claude/`, OpenCode `~/.config/opencode/`) without exposing host `HOME`.
- Added `CLAUDE_CONFIG_DIR` environment variable injection when Claude Code config directories are mounted via `--mount-agent-config`.
- Added Claude Code built-in egress endpoint profile (`api.anthropic.com`, `claude.ai`, `platform.claude.com`) for `--egress auto` allowlist resolution.
- Added OpenCode provider-aware egress endpoint profile (`models.dev`, resolved provider host, conditional `opencode.ai`) for `--egress auto` allowlist resolution.
- Added pre-start provider resolution for OpenCode under `--egress auto` that reads auth and config state to determine the required provider host, failing before container start with actionable guidance when the host cannot be determined.
- Added model-aware provider resolution for OpenCode `--egress auto`: when `--model provider/model` specifies a known provider whose API host differs from the configured provider, the built-in allowlist now includes both hosts; unknown provider prefixes fail before container start with `--egress-allow` guidance.
- Added agent auto-update via cache-aware init container: by default, `tessariq run` attempts to update the selected agent to the latest version before each run using a short-lived init container. Updated binaries are cached at `~/.tessariq/agent-cache/<agent>/` and mounted read-only into the agent container with PATH layering. Update failures fall back to the baked version with a warning. Use `--no-update-agent` to skip the init phase.
- Added built-in baseline package-manager allowlist profile (npm, PyPI, RubyGems, crates.io, Go module proxy/checksum, Maven Central, Wikipedia) for `--egress auto` and `--egress proxy` modes.
- Added user-level config loading from `$XDG_CONFIG_HOME/tessariq/config.yaml` (or `~/.config/tessariq/config.yaml`) for default proxy allowlist selection.
- Added `--egress-no-defaults` behavior that discards built-in and user-configured default allowlists, requiring explicit `--egress-allow` entries.
- Added allowlist entry validation with hostname and port range checks, defaulting to port 443 when omitted.
- Added full allowlist precedence resolution: CLI `--egress-allow` overrides user config, which overrides the built-in profile; `allowlist_source` in `manifest.json` now records the exact provenance (`cli`, `user_config`, or `built_in`).
- Added Docker container isolation for agent execution: agent binaries run inside a Docker container with the worktree mounted read-write at `/work`, evidence at `/evidence`, auth files read-only at deterministic paths under `/home/tessariq/`, and optional config directories when `--mount-agent-config` is used.
- Added Docker as a required host prerequisite for `tessariq run` with daemon-reachability preflight check and actionable guidance when Docker is missing or not running.
- Added proxy-mode network topology: agent containers run on an internal Docker network with egress enforced through a per-run Squid proxy that allowlists destinations at `host:port` granularity via CONNECT tunneling.
- Added `egress.compiled.yaml` evidence artifact emitted in proxy mode with `schema_version`, `allowlist_source`, and fully resolved `host:port` destinations.
- Added `egress.events.jsonl` evidence artifact emitted in proxy mode recording blocked egress attempts with timestamp, destination, and reason.
- Added blocked-destination UX: when proxy mode blocks egress, the CLI reports which `host:port` was denied and how to allow it via `--egress-allow`, user config, or `--unsafe-egress`.
- Added `tessariq promote <run-ref>` with repo-scoped run resolution, zero-diff protection, default Tessariq commit trailers, and one-branch/one-commit promotion from captured `diff.patch` evidence.
- Added `tessariq attach <run-ref>` with shared repo-scoped run resolution for `run_id`, `last`, and `last-N`, live-run-only eligibility checks, and minimal detach guidance (`Ctrl-b d`) in the command help.

### Security

- Added symlink resolution to task-path validation so `tessariq run` rejects symlinks whose real target escapes the repository boundary, preventing external Markdown files from being smuggled into evidence.
- Added symlink resolution to evidence-path validation so `tessariq attach` and `tessariq promote` reject run evidence directories whose real target escapes `.tessariq/runs/`, closing a path-escape via symlinks planted under the runs tree.
- Added container security hardening: agent containers are now created with `--cap-drop=ALL` and `--security-opt=no-new-privileges` to drop all Linux capabilities and prevent privilege escalation.
- Changed evidence file permissions from world-readable (`0644`/`0755`) to owner-only (`0600`/`0700`) for all evidence directories and files.
- Pinned workspace repair container image by digest (`alpine@sha256:...`) instead of mutable `alpine:latest` tag to prevent supply-chain attacks on the root-privileged ownership-fix container.
- Changed `tessariq init` to create `.tessariq/` and `.tessariq/runs/` with owner-only permissions (`0700`) and tighten existing directories on re-run.
- Reject allowlist hosts containing ASCII control characters (NUL, newline, carriage return, and others) to prevent Squid proxy config injection via malformed `--egress-allow` values or user config entries.
- Reject allowlist hosts with a leading dot (e.g. `.example.com`) to prevent Squid `dstdomain` wildcard matching that would widen a single host entry into a subdomain wildcard.
- Added container security hardening to the Squid proxy container: `--cap-drop=ALL`, `--cap-add=SETGID`, `--cap-add=SETUID`, and `--security-opt=no-new-privileges` so the egress boundary matches the agent container's baseline restrictions.
- Restored the v0.1.0 read-only host auth mount contract: Claude Code's `~/.claude.json` is no longer bind-mounted writable into the agent container. Tessariq now materializes a disposable per-run runtime-state file under `~/.tessariq/runtime-state/<run_id>/` seeded from the host source, bind-mounts that per-run copy read-write at the agent's expected container path, and removes it after the run. In-container writes to `~/.claude.json` can no longer persist to the live host auth file, closing the container-to-host persistence attack surface reachable via MCP server injection. OpenCode auth and config mounts are unaffected. `runtime.json.auth_mount_mode` is now derived from the actual mount assembly and enforced by a shared `authmount.ValidateContract` invariant so future adapters cannot silently reintroduce a writable host bind.
- Pinned default Squid proxy image by digest (`ubuntu/squid@sha256:...`) instead of mutable `ubuntu/squid:latest` tag to prevent supply-chain drift on the egress-boundary container.
- Pinned default Claude Code and OpenCode reference agent images by digest instead of mutable `:latest` tags, completing supply-chain pinning for all first-party runtime images.
- Added reference agent image Dockerfiles (`runtime/claude-code/`, `runtime/opencode/`) and CI workflow for automated image building, testing, and publishing with vulnerability scanning. These images are for quick onboarding and experimentation; production users should bring their own runtime images via `--image`.

- Added `--interactive` support for the OpenCode adapter so `tessariq run --agent opencode --interactive` launches OpenCode in TUI mode with full TTY allocation and direct container attach via `--attach`.
- Added `--model` passthrough for the OpenCode adapter so `tessariq run --agent opencode --model <provider/model>` forwards the model identifier to the OpenCode CLI. Previously the flag was silently dropped.
- Added formal `Agent` interface in the adapter package, replacing duplicated per-agent field extraction in the factory with a single interface-based dispatch.

### Changed

- Changed agent binary validation from reactive (container exit code 127) to proactive pre-start detection: `tessariq run` now probes the resolved runtime image for the selected agent binary before starting the container, failing with an actionable message that names the missing binary, the selected agent, and `--image` override guidance.
- Changed log capping from post-run file truncation to write-time enforcement via `CappedWriter`, so `run.log` and `runner.log` never exceed the configured limit during execution and include a `[truncated]` marker when capped.
- Changed agent execution model from direct host `exec.CommandContext` to Docker container lifecycle (`docker create` + `docker start` + `docker wait` + `docker rm`) with deterministic container names (`tessariq-<run_id>`) and cleanup on all exit paths.

- Changed evidence model from single `adapter.json` to split `agent.json` (requested/supported options) and `runtime.json` (image identity and mount-policy metadata), aligning with v0.1.0 agent-and-runtime contract.
- Renamed `agent.json.applied` to `agent.json.supported` to reflect its agent-capability-map semantics; the spec example and adapter comments now show `requested.interactive=false` alongside `supported.interactive=true` when interactive support exists.
- Changed `manifest.json` to use `agent` instead of `adapter` and added `resolved_egress_mode` and `allowlist_source` fields per v0.1.0 spec.
- Changed CLI approval and egress flag UX: replaced `--yolo` with `--interactive` (autonomous-by-default) and renamed `--egress-allow-reset` to `--egress-no-defaults` for clearer intent.
- Changed prerequisite preflight UX for local CLI execution so `tessariq init`, `tessariq run`, and `tessariq attach` fail fast with actionable missing-dependency guidance before lifecycle side effects.
- Changed `tessariq run --interactive` from a blanket rejection to full interactive runtime support: containers are created with TTY allocation, the tmux session attaches to the container for live terminal input, and the active-agent timeout pauses while the agent waits for human approval instead of ticking wall-clock time. OpenCode rejects `--interactive` with actionable guidance; Claude Code supports it natively.

### Fixed

- Fixed `tessariq run --attach --interactive` hanging after the Claude Code trust prompt due to a double-PTY chain (tmux pane PTY nested inside container PTY); interactive attach now connects the terminal directly to the container via `docker attach`, eliminating the nested PTY.
- Fixed `tessariq run --interactive` not passing the task content to Claude Code; the task is now pre-loaded as an initial prompt in interactive mode.
- Fixed `tessariq run` exiting zero and printing success-style output for terminal non-success outcomes (failed, timeout, killed, interrupted); the CLI now exits non-zero with state and evidence path guidance for non-success runs.
- Fixed `diff.patch` silently dropping binary file changes because `git diff` was invoked without `--binary`, causing `tessariq promote` to lose binary additions and modifications.
- Fixed `tessariq promote` trusting forged `evidence_path` values from the run index, now rejecting absolute paths and relative paths that escape the repository's `.tessariq/runs/` directory before reading any evidence files.
- Fixed `tessariq promote` accepting changed runs that are missing `diffstat.txt`, now requiring both `diff.patch` and `diffstat.txt` as the spec mandates.
- Fixed `tessariq promote` accepting proxy-mode runs whose required egress artifacts were missing; completeness now requires non-empty `egress.compiled.yaml` and `egress.events.jsonl` whenever `resolved_egress_mode=proxy`, with the same `required evidence is missing or incomplete …` guidance used by other evidence gates. Non-proxy runs are unaffected.
- Fixed worktree cleanup aborting when the Docker ownership-repair container is unavailable, leaving stale git worktree refs and orphaned directories; cleanup now continues with a host-side chmod fallback and best-effort teardown.
- Fixed timeout signal escalation to send SIGTERM before SIGKILL, giving containers a grace period for clean shutdown before forced termination.
- Fixed `--grace` flag being ignored for container-backed runs because `docker stop --time=10` hardcoded a 10-second grace period; container SIGTERM now uses non-blocking `docker kill --signal=SIGTERM` so the runner's own grace timer controls escalation.
- Added `--init` to container creation so tini runs as PID 1 and forwards signals to the agent process, ensuring SIGTERM is reliably delivered regardless of whether the agent registers a signal handler.
- Fixed `--agent opencode --interactive` being rejected at CLI validation instead of proceeding with requested/supported evidence recording; runs now preserve `requested.interactive=true`, and `supported.interactive` continues to record the adapter's interactive capability.
- Fixed `--egress-allow` being ignored for OpenCode when provider auto-resolution fails: explicit CLI allowlist entries now take precedence, skipping provider detection entirely so runs proceed without requiring auth state.
- Fixed OpenCode proxy runs failing when user-config `egress_allow` is present but `auth.json` lacks provider info: provider resolution is now skipped when a higher-precedence allowlist source (CLI or user config) already determines egress destinations.
- Fixed OpenCode proxy runs surfacing raw filesystem errors when auth state is missing: provider resolution now returns actionable guidance telling the user to authenticate OpenCode locally first.
- Fixed duration default rendering in `--help` output so `--timeout` and `--grace` show normalized values (for example `30m` and `30s`) instead of padded forms.
- Fixed detached run sessions so the host tmux session tails durable `run.log` output from the container instead of starting empty.
- Fixed worktree cleanup after container-owned writes by repairing disposable workspace ownership before worktree removal.
- Fixed leaked worktree directories and stale git worktree entries when a run fails after worktree provisioning.
- Fixed potential `base_sha` divergence between `manifest.json` and `workspace.json` by resolving HEAD once and passing it through workspace provisioning.
- Fixed silent `index.jsonl` append failures so manifest, status, and file-write errors now emit a `warning:` line to stderr instead of being swallowed.
- Fixed spurious "interactive mode without --attach" note on default runs by gating the note on the user's explicit `--interactive` flag instead of the agent's capability declaration.
- Fixed `tessariq attach` missing `git` from its prerequisite preflight, causing a raw exec error instead of actionable guidance when git is unavailable.
- Fixed `tessariq attach` trusting forged `evidence_path` values from the run index, now rejecting absolute paths and relative paths that escape the repository's `.tessariq/runs/` directory before reading live-run evidence.
- Fixed `tessariq attach` allowing one run to borrow another run's evidence directory for the liveness check; evidence directory name must now match the resolved run ID.
- Fixed `tessariq promote` accepting changed runs that are missing `diffstat.txt`, now requiring both `diff.patch` and `diffstat.txt` as the spec mandates.
- Fixed `attach` and `promote` acting on semantically incomplete index entries by validating all required fields during index read; entries missing any of the eight minimum fields are now silently skipped.
- Fixed silent `WriteDiffArtifacts` failure so diff generation errors now emit a `warning:` line to stderr instead of being discarded.
- Fixed `last` and `last-N` run-ref resolution counting raw index lines instead of unique runs, causing `last-1` to resolve to a lifecycle entry of the same run rather than the previous run when multiple entries exist for one run.
- Fixed `--egress none` leaving containers on Docker's default bridge network with full internet access; containers now run with `--net none` (loopback only) as intended.
- Fixed `tessariq promote` trusting `manifest.json` identity fields without verifying they match the resolved run, allowing a tampered manifest to forge branch names and commit trailers for a different run.
- Fixed `--egress open`, `--egress none`, and `--egress proxy --egress-allow` failing on malformed or unreadable user config even though the CLI already fully determines egress behavior; user config is now loaded only when it can influence the resolved allowlist.
- Fixed proxy-mode Squid ACLs allowing the cross-product of all allowed hosts and all allowed ports; each allowlist entry now authorizes only its exact host-port pair.
- Fixed `--egress open --egress-allow` silently ignoring allowlist entries; the combination is now rejected at validation time with an actionable error directing users to proxy mode.
- Fixed `manifest.json` writes using non-atomic `os.WriteFile` which could leave partially written JSON on crash; writes now use the same temp-file-plus-rename pattern as other evidence files.
- Fixed proxy startup failure leaving orphaned Squid containers and Docker networks; `StartSquid` now removes the created container on any post-create failure, and `Topology.Setup` removes both container and network before returning the error.
- Fixed `ParseDestination` corrupting IPv6 addresses by splitting on the last colon; bracketed IPv6 forms like `[::1]:443` now parse correctly, and bare IPv6 addresses are rejected with actionable guidance to use the bracketed form.
- Fixed `run.log` missing agent output emitted during grace-period shutdown after timeout; log streaming now survives timeout context cancellation and drains until the container exits.
- Fixed `LoadUserConfig` silently ignoring unknown YAML keys (e.g. `egressAllow` instead of `egress_allow`), causing typos to widen egress by falling back to built-in allowlists; unknown fields now fail with an actionable error identifying the config path and invalid key.
- Fixed `--pre` and `--verify` hooks executing with the evidence directory as working directory instead of the repository root, causing relative-path project commands like `go test ./...` to fail without an inline `cd` workaround.
- Fixed `tessariq run` returning only the error string on post-bootstrap failures without printing `run_id` or `evidence_path`, forcing users to search for evidence artifacts manually.
- Fixed detached runs being left indefinitely `running` after host-side interruption or supervisor loss: the CLI now preserves terminal status on SIGINT/SIGTERM, defers container removal until after terminal evidence is written, and reconciles orphaned `running` runs during `attach` and `promote` so `status.json` and `index.jsonl` converge on a non-running terminal state.
