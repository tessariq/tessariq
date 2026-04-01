# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added `tessariq init` repository bootstrap that creates `.tessariq/runs/` and idempotently updates `.gitignore` to keep runtime state untracked.
- Added `tessariq run <task-path>` CLI wiring with initial flag surface, task-path validation, and manifest bootstrap with stable preflight fields.
- Added pre-run clean-repository gating and task ingestion so runs fail early on dirty repos and record copied task context (`task.md`) plus derived task title metadata.
- Added detached worktree provisioning under `~/.tessariq/worktrees/<repo_id>/<run_id>` and `workspace.json` evidence with reproducibility-focused metadata.
- Added runner lifecycle evidence contracts, including durable `status.json`, `run.log`, `runner.log`, deterministic container naming, and timeout bookkeeping.
- Added detached-by-default tmux session startup with script-friendly stdout guidance for `attach` and `promote` commands.
- Added shared adapter evidence contract with `adapter.json` requested-versus-applied recording semantics for exact and partial option application.
- Added first-party `claude-code` adapter support integrated into run lifecycle execution and evidence output.
- Added first-party `opencode` adapter with partial-application recording for unsupported `--model` and `--interactive` options.
- Added actionable binary-not-found error messages for both `claude-code` and `opencode` adapters naming the missing binary and container image expectation.
- Added per-agent auth discovery that auto-detects required auth files before agent start and fails with actionable messages when auth is missing, Keychain-only on macOS, or writable refresh is required.
- Added `--mount-agent-config` flag to `tessariq run` for opt-in read-only mounting of supported agents' default config directories (Claude Code `~/.claude/`, OpenCode `~/.config/opencode/`) without exposing host `HOME`.
- Added `CLAUDE_CONFIG_DIR` environment variable injection when Claude Code config directories are mounted via `--mount-agent-config`.
- Added Claude Code built-in egress endpoint profile (`api.anthropic.com`, `claude.ai`, `platform.claude.com`) for `--egress auto` allowlist resolution.
- Added OpenCode provider-aware egress endpoint profile (`models.dev`, resolved provider host, conditional `opencode.ai`) for `--egress auto` allowlist resolution.
- Added pre-start provider resolution for OpenCode under `--egress auto` that reads auth and config state to determine the required provider host, failing before container start with actionable guidance when the host cannot be determined.
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

- Added container security hardening: agent containers are now created with `--cap-drop=ALL` and `--security-opt=no-new-privileges` to drop all Linux capabilities and prevent privilege escalation.
- Changed evidence file permissions from world-readable (`0644`/`0755`) to owner-only (`0600`/`0700`) for all evidence directories and files.
- Pinned workspace repair container image by digest (`alpine@sha256:...`) instead of mutable `alpine:latest` tag to prevent supply-chain attacks on the root-privileged ownership-fix container.
- Changed `tessariq init` to create `.tessariq/` and `.tessariq/runs/` with owner-only permissions (`0700`) and tighten existing directories on re-run.

### Changed

- Changed agent binary validation from reactive (container exit code 127) to proactive pre-start detection: `tessariq run` now probes the resolved runtime image for the selected agent binary before starting the container, failing with an actionable message that names the missing binary, the selected agent, and `--image` override guidance.
- Changed log capping from post-run file truncation to write-time enforcement via `CappedWriter`, so `run.log` and `runner.log` never exceed the configured limit during execution and include a `[truncated]` marker when capped.
- Changed agent execution model from direct host `exec.CommandContext` to Docker container lifecycle (`docker create` + `docker start` + `docker wait` + `docker rm`) with deterministic container names (`tessariq-<run_id>`) and cleanup on all exit paths.

- Changed evidence model from single `adapter.json` to split `agent.json` (requested/applied options) and `runtime.json` (image identity and mount-policy metadata), aligning with v0.1.0 agent-and-runtime contract.
- Changed `manifest.json` to use `agent` instead of `adapter` and added `resolved_egress_mode` and `allowlist_source` fields per v0.1.0 spec.
- Changed CLI approval and egress flag UX: replaced `--yolo` with `--interactive` (autonomous-by-default) and renamed `--egress-allow-reset` to `--egress-no-defaults` for clearer intent.
- Changed prerequisite preflight UX for local CLI execution so `tessariq init`, `tessariq run`, and `tessariq attach` fail fast with actionable missing-dependency guidance before lifecycle side effects.
- Changed `tessariq run --interactive` from a blanket rejection to full interactive runtime support: containers are created with TTY allocation, the tmux session attaches to the container for live terminal input, and the active-agent timeout pauses while the agent waits for human approval instead of ticking wall-clock time. OpenCode rejects `--interactive` with actionable guidance; Claude Code supports it natively.

### Fixed

- Fixed timeout signal escalation to send SIGTERM before SIGKILL, giving containers a grace period for clean shutdown before forced termination.
- Fixed `--agent opencode --interactive` being rejected at CLI validation instead of proceeding with requested/applied evidence recording; `agent.json` now correctly records `requested.interactive=true` and `applied.interactive=false`.
- Fixed `--egress-allow` being ignored for OpenCode when provider auto-resolution fails: explicit CLI allowlist entries now take precedence, skipping provider detection entirely so runs proceed without requiring auth state.
- Fixed duration default rendering in `--help` output so `--timeout` and `--grace` show normalized values (for example `30m` and `30s`) instead of padded forms.
- Fixed detached run sessions so the host tmux session tails durable `run.log` output from the container instead of starting empty.
- Fixed worktree cleanup after container-owned writes by repairing disposable workspace ownership before worktree removal.
- Fixed leaked worktree directories and stale git worktree entries when a run fails after worktree provisioning.
- Fixed potential `base_sha` divergence between `manifest.json` and `workspace.json` by resolving HEAD once and passing it through workspace provisioning.
