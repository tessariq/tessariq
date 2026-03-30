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

### Changed

- Changed CLI approval and egress flag UX: replaced `--yolo` with `--interactive` (autonomous-by-default) and renamed `--egress-allow-reset` to `--egress-no-defaults` for clearer intent.
- Changed prerequisite preflight UX for local CLI execution so `tessariq init`, `tessariq run`, and `tessariq attach` fail fast with actionable missing-dependency guidance before lifecycle side effects.

### Fixed

- Fixed duration default rendering in `--help` output so `--timeout` and `--grace` show normalized values (for example `30m` and `30s`) instead of padded forms.
