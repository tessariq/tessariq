# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Proxy mode now prints the "Blocked egress destinations" guidance for the run that was just executed. The proxy was torn down — and its egress telemetry extracted — only after the guidance was printed, so blocked destinations and the `--egress-allow` hint never appeared.
- Proxy topology teardown is now bounded by a 30-second timeout. An unresponsive Docker daemon could previously hang `tessariq run` indefinitely while dismantling the Squid container and per-run network — including after Ctrl+C, which is exactly when a user wants out. Teardown failures are still reported as a warning and never change the run's terminal state.

## [0.1.0] - 2026-07-30

First release. Tessariq runs a coding agent against a Markdown task in an
isolated workspace and promotes the result as one reviewable commit.

### Added

- **Commands.** `tessariq init` bootstraps `.tessariq/runs/` and keeps runtime state untracked. `tessariq run <task.md>` executes a task and returns immediately with a `run_id`, evidence path, and copy-paste `attach`/`promote` commands. `tessariq attach <run-ref>` joins a live run's tmux session; `run --attach` does it in one step. `tessariq promote <run-ref>` lands the result. `tessariq version` and `--version` print the build version without needing a repository.
- **Run isolation.** Each run gets a detached Git worktree under `~/.tessariq/worktrees/` and a dedicated Docker container, so your working tree is never mutated. Runs refuse to start on a dirty repository.
- **Agents.** First-party `claude-code` and `opencode` adapters, both supporting `--model` forwarding and `--interactive` TUI mode. `agent.json` records requested-versus-supported options so partial support is visible rather than silent.
- **Auth reuse.** Per-agent discovery of existing local credentials, mounted read-only at deterministic container paths without exposing host `HOME`. Missing, Keychain-only, or unreadable auth fails before the container starts with actionable guidance. `--mount-agent-config` optionally mounts the agent's config directory read-only.
- **Agent auto-update.** A short-lived init container updates the agent before each run, caching binaries under `~/.tessariq/agent-cache/` and falling back to the image's baked version on failure. `--no-update-agent` skips it.
- **Egress control.** `--egress auto|proxy|open|none`. Proxy mode routes the container through a per-run Squid proxy that allowlists destinations at `host:port` granularity and records every blocked attempt. Built-in profiles cover each agent's API plus common package registries. `--egress-allow`, user config at `~/.config/tessariq/config.yaml`, and `--egress-no-defaults` compose with a documented precedence recorded in `allowlist_source`.
- **Evidence.** Every run writes `manifest.json`, `status.json`, `agent.json`, `runtime.json`, `workspace.json`, `task.md`, `run.log`, and `runner.log`, plus `diff.patch`/`diffstat.txt` when code changed and `egress.compiled.yaml`/`egress.events.jsonl` in proxy mode. Artifacts are plain JSON, YAML, Markdown, and text, written atomically and completed even when a run fails.
- **Promotion safety.** `promote` produces exactly one branch and one commit with `Tessariq-*` trailers, refuses runs with no diff, and validates that evidence is present, parseable, and internally consistent before touching Git.
- **Run controls.** `--timeout` and `--grace` bound the whole run including hooks, escalating SIGTERM to SIGKILL. `--pre` and `--verify` hooks run on the host from the repository root. `--image` overrides the runtime image.
- **Runtime images.** Published reference runtime and agent images under `ghcr.io/tessariq/`, built, vulnerability-scanned, and pushed by CI. Used by default so a first run needs no image build; derive your own for production.

### Security

- Agent containers run as a non-root user with `--cap-drop=ALL` and `--security-opt=no-new-privileges`; the Squid proxy container is hardened to the same baseline.
- Host auth state is mounted read-only. Claude Code's `~/.claude.json` is copied to a disposable per-run file so in-container writes — including MCP server injection — cannot persist to the live host credential file.
- Evidence directories and files are owner-only (`0700`/`0600`), and live worktrees are readable only by the invoking user and the container's `tessariq` user.
- Path handling resolves symlinks so task paths and evidence directories cannot escape the repository, and `promote`/`attach` reject forged `evidence_path` values from the run index.
- Task paths and allowlist hosts reject ASCII control characters, preventing forged commit trailers and Squid config injection. Allowlist hosts reject a leading dot so one entry cannot widen into a subdomain wildcard.
- `promote` rejects tampered evidence: forged run identity, mismatched egress mode, and malformed structured artifacts are all refused.
- Infrastructure images are pinned by digest (`alpine`, `ubuntu/squid`); default agent images are pinned to immutable version tags.

[Unreleased]: https://github.com/tessariq/tessariq/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/tessariq/tessariq/releases/tag/v0.1.0
