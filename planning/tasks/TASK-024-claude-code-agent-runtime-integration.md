---
id: TASK-024-claude-code-agent-runtime-integration
title: Integrate Claude Code with the v0.1.0 agent and runtime model
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#tessariq-run-task-path
dependencies:
    - TASK-021-reference-runtime-image-and-docs
    - TASK-022-agent-and-runtime-evidence-migration
    - TASK-023-supported-agent-auth-mounts
    - TASK-026-mount-agent-config-flag-and-config-dir-mounts
updated_at: "2026-03-31T08:45:39Z"
---

## Summary

Integrate `claude-code` with the v0.1.0 agent/runtime model, including runtime-binary validation, read-only auth reuse, and the new evidence contract.

## Acceptance Criteria

- `agent.json` records `agent=claude-code` and the requested/applied option semantics required by the active spec.
- Claude Code integrates cleanly with the run lifecycle.
- Tessariq validates that the `claude` binary is present in the resolved runtime image before agent start.
- Missing-Claude-Code-binary failures identify the missing `claude` binary and tell the user to use a compatible runtime image or `--image` override.
- Claude Code works with the supported read-only auth-mount contract, including Linux file-backed auth and the macOS file-backed credential-mirror requirement.
- When Claude Code config directories are mounted, Tessariq sets `CLAUDE_CONFIG_DIR=$HOME/.claude` inside the container.
- Claude Code uses exactly `api.anthropic.com:443`, `claude.ai:443`, and `platform.claude.com:443` under `--egress auto` in addition to the baseline package-manager allowlist.

## Test Expectations

- Add unit tests for command/option translation, `claude` runtime-binary validation, and `CLAUDE_CONFIG_DIR` environment wiring.
- Add integration tests for real agent invocation using Testcontainers-backed collaborators only.
- Add integration tests for missing-binary and missing-auth error handling, including the macOS credential-mirror failure path.
- Add integration tests for agent process crash mid-run.
- Add a thin e2e run path once the agent is wired into run execution.
- Run mutation testing because option translation is branchy.

## TDD Plan

- Start with a failing unit test for Claude Code option translation and missing-`claude`-binary validation.

## Notes

- This task supersedes the old adapter-specific implementation task without rewriting that completed task.
- 2026-03-31T08:45:39Z: BinaryName constants, CLAUDE_CONFIG_DIR env var wiring, Claude Code egress endpoints, full test coverage, 100% mutation efficacy. Local-only verification artifact generated.
