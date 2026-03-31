# Manual Test Report

Task: `TASK-029-interactive-runtime-mode-independent-of-attach`
Date: 2026-03-31T17:29:49Z

## Results

| Step | Description | Result |
|------|-------------|--------|
| 1 | Build binary and test environment | PASS |
| 2 | Build fake agent Docker image | PASS |
| 3 | Init git repo with sample task | PASS |
| 4 | `tessariq run --interactive` succeeds | PASS |
| 4a | Exit code is 0, output includes run_id etc. | PASS |
| 4b | agent.json: `requested.interactive: true`, `applied.interactive: true` | PASS |
| 4c | status.json: `state: success`, `exit_code: 0`, `timed_out: false` | PASS |
| 4d | stderr includes "note: interactive mode without --attach" warning | PASS |
| 5 | `--agent opencode --interactive` fails with "not supported by opencode" | PASS |
| 6 | Non-interactive detached run still works unchanged | PASS |
| 7 | Cleanup | PASS |

## Evidence

Interactive run agent.json:
```json
{
    "schema_version": 1,
    "agent": "claude-code",
    "requested": {"interactive": true},
    "applied": {"interactive": true}
}
```

Interactive run status.json:
```json
{
    "schema_version": 1,
    "state": "success",
    "started_at": "2026-03-31T17:31:57Z",
    "finished_at": "2026-03-31T17:31:57Z",
    "exit_code": 0,
    "timed_out": false
}
```

OpenCode rejection:
```
--interactive is not supported by opencode; use --agent claude-code for interactive mode
EXIT=1
```

## Conclusion

All acceptance criteria verified. Interactive mode starts successfully with Claude Code, records correct agent metadata, fails with actionable guidance for OpenCode, and detached (non-interactive) runs remain unaffected.
