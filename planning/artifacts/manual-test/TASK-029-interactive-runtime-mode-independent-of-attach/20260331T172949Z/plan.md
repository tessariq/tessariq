# Manual Test Plan

Task: `TASK-029-interactive-runtime-mode-independent-of-attach`

## Scope

- Verify `tessariq run --interactive` starts the agent in interactive mode with TTY.
- Verify the tmux session attaches to the container (not just tails run.log).
- Verify agent.json records `requested.interactive: true` and `applied.interactive: true` for Claude Code.
- Verify `--interactive` with `--agent opencode` fails with actionable guidance.
- Verify the activity-based timeout pauses during idle periods (tested via unit/integration tests; manual verification observes that a short-timeout interactive run with a sleeping agent does not expire prematurely).
- Verify detached (non-interactive) runs still work unchanged.

## Environment

- Built `tessariq` locally with `CGO_ENABLED=0 go build -o /tmp/tessariq-manual-task029/tessariq ./cmd/tessariq`
- Temporary repo: `/tmp/tessariq-manual-task029/repo`
- Temporary HOME with fake Claude auth: `/tmp/tessariq-manual-task029/home`
- Fake agent image: `tessariq-manual-agent-task029`

## Steps

1. Build the tessariq binary and create a temporary test environment.
2. Build a fake agent Docker image with a script that echoes output and exits.
3. Initialize a git repo with a sample task file and commit.
4. Run `tessariq run --interactive --image tessariq-manual-agent-task029 tasks/sample.md` and verify:
   - Exit code is 0
   - Output includes `run_id`, `evidence_path`, etc.
   - `agent.json` has `requested.interactive: true` and `applied.interactive: true`
   - stderr includes the "note: interactive mode without --attach" warning
5. Run `tessariq run --agent opencode --interactive --egress none --image tessariq-manual-agent-task029 tasks/sample.md` and verify it fails with "not supported by opencode".
6. Run a non-interactive detached run and verify it still works correctly.
7. Clean up temporary files and Docker images.
