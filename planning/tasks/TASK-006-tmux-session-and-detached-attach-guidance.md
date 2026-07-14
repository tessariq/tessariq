---
id: TASK-006-tmux-session-and-detached-attach-guidance
title: Start tmux sessions and print detached attach guidance
status: completed
priority: medium
spec_ref: specs/tessariq-v0.1.0.md#product-intent
dependencies:
    - TASK-005-runner-bootstrap-timeout-and-status-lifecycle
updated_at: "2026-03-30T15:21:26Z"
---

## Summary

Launch live `tmux` sessions for runs and print attach and promote guidance in detached mode.

## Acceptance Criteria

- Runs remain detached by default with `--attach=false`, while still creating a live `tmux` session that later attach flows can target.
- Printed output includes run id, workspace path, evidence path, container identifier, attach command, and promote command.
- Tmux session naming and attach-command generation are stable enough for later attach behavior.
- Stdout output remains script-friendly while still surfacing the attach and promote commands.

## Test Expectations

- Add unit tests for printed output formatting and command hints, with individual assertions that each required field appears in stdout: `run_id`, workspace path, evidence path, container name, attach command, and promote command.
- Add unit tests for tmux-not-available error handling (clean failure with user guidance) and session name collision behavior.
- Add unit tests verifying stdout/stderr separation: user-facing output on stdout (script-friendly), diagnostics on stderr only.
- Add integration tests for tmux session creation using Testcontainers-backed collaborators only.
- Add a thin e2e test for the detached `run -> attach guidance` user experience once the command is runnable.
- Mutation testing is optional unless session lifecycle branching grows.

## TDD Plan

- Start with a failing unit test for required printed output and a failing e2e expectation for detached guidance once runnable.

## Notes

- Keep stdout output script-friendly.
- 2026-03-30T15:21:26Z: evidence: local-only verification artifact (0 findings), local-only manual-test artifact (9/9 pass)
