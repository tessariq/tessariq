---
id: TASK-078-fix-interactive-attach-double-pty-and-task-passthrough
title: Fix interactive attach double-PTY hang and pass task content
status: done
priority: p1
depends_on:
    - TASK-071-implement-run-attach-live-session
    - TASK-029-interactive-runtime-mode-independent-of-attach
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#tessariq-run-task-path
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
updated_at: "2026-04-05T13:03:04Z"
areas:
    - cli
    - runner
    - tmux
    - interactive
    - adapter
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Adapter arg building and runner session signaling changes require precise unit coverage.
    integration:
        required: false
        commands: []
        rationale: The double-PTY fix changes the attach path but uses the same container and tmux primitives already covered.
    e2e:
        required: false
        commands: []
        rationale: The fix changes plumbing between existing components; manual testing covers the real UX.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Conditional branching on interactive mode is easy to partially wire.
    manual_test:
        required: true
        commands: []
        rationale: The interactive attach TUI flow can only be verified manually with a live container.
---

## Summary

`tessariq run <task.md> --attach --interactive` has two bugs:

1. **Terminal hangs after trust prompt.** The interactive tmux session runs `docker attach <container>` inside a tmux pane, creating a double-PTY chain (`user terminal -> tmux pane PTY -> docker attach -> container PTY`). Terminal escape sequences (cursor position queries) get lost in the nested PTY chain, causing Claude Code's TUI to freeze.

2. **Task content not passed in interactive mode.** `buildArgs()` in the Claude Code adapter skips the task content when `Interactive` is true. Claude starts without any task loaded. OpenCode is not affected (always passes task content).

## Acceptance Criteria

- `tessariq run <task.md> --attach --interactive` starts Claude Code with the task content pre-loaded as the initial prompt.
- The interactive attach flow uses direct `docker attach` from the user's terminal, eliminating the double-PTY chain.
- A tmux session is still created for log tailing (accessible via `tmux attach -t <session>`), but is not the primary interaction path for interactive mode.
- Non-interactive `--attach` behavior is unchanged (still attaches to tmux session).
- OpenCode adapter behavior is unchanged (already passes task content; does not support interactive mode natively).

## Test Expectations

- Unit tests for Claude Code adapter verify that task content is included in interactive mode args.
- Unit tests for runner verify that `SessionReady` fires after process start in interactive mode (before tmux session creation).
- Unit tests for `runWithAttach` verify that interactive mode dispatches to direct container attach instead of tmux attach.
- Existing non-interactive tests remain green.

## TDD Plan

1. RED: add failing test that interactive `buildArgs` includes task content.
2. GREEN: fix `buildArgs` to always include task content.
3. RED: add failing test that interactive `SessionReady` fires after process start, before tmux session.
4. GREEN: restructure `runInteractiveProcess` to signal ready after start, use log-tailing tmux session.
5. RED: add failing test that interactive attach dispatches `docker attach` directly.
6. GREEN: add `attachContainerFn` and dispatch in `run.go`.

## Notes

- The `sessionCommand` method's interactive branch (`docker attach <container>`) can be removed since interactive mode will use log-tailing tmux sessions (same as non-interactive).
- `Runner.ContainerName` is still needed for evidence recording but no longer for session command construction.
- The `--dangerously-skip-permissions` flag is intentionally NOT passed in interactive mode; interactive means the user approves tool invocations.
- 2026-04-05T13:03:04Z: Fixed interactive attach double-PTY hang and task content passthrough. Direct docker attach eliminates nested PTY. Task content always passed as initial prompt. Manual tests: 7/7 pass. Mutation efficacy: 85.77%.
