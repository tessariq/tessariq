# Manual Test Report

- Task: TASK-026-mount-agent-config-flag-and-config-dir-mounts
- Executed: 2026-03-31T10:00:00Z
- Verdict: pass

## Results

### MT-001: --mount-agent-config flag exists with default false

- Status: pass
- Observation: `tessariq run --help` shows `--mount-agent-config` with description "mount the agent's default config directory read-only". No default value printed (bool defaults to false).

### MT-002: DefaultConfig has MountAgentConfig false

- Status: pass
- Observation: `run.DefaultConfig().MountAgentConfig` is false.

### MT-003: DiscoverConfigDirs returns mounted for Claude Code with existing dir

- Status: pass
- Observation: Status "mounted", 1 mount, container path /home/tessariq/.claude, ReadOnly true.

### MT-004: DiscoverConfigDirs returns mounted for OpenCode with existing dir

- Status: pass
- Observation: Status "mounted", 1 mount, container path /home/tessariq/.config/opencode, ReadOnly true.

### MT-005: DiscoverConfigDirs returns missing_optional when dir absent

- Status: pass
- Observation: Status "missing_optional", 0 mounts, no error returned.

### MT-006: runtime.json records correct agent_config_mount values

- Status: pass
- Observation: All four combinations (disabled/disabled, enabled/mounted, enabled/missing_optional, enabled/unreadable_optional) serialize correctly to JSON with correct field names and values.

### MT-007: Config dir mounts do not expose host HOME

- Status: pass
- Observation: No mount for either agent has ContainerPath equal to the host home directory.

### MT-008: CLAUDE_CONFIG_DIR env var set when Claude Code config mounted

- Status: pass
- Observation: EnvVars["CLAUDE_CONFIG_DIR"] = "/home/tessariq/.claude" when config dir is mounted.

## Summary

- Total: 8 | Pass: 8 | Fixed: 0 | Failed: 0 | Skipped: 0
