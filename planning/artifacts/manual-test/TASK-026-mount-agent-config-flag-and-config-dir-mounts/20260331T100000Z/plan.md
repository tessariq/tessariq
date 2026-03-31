# Manual Test Plan

- Task: TASK-026-mount-agent-config-flag-and-config-dir-mounts
- Generated: 2026-03-31T10:00:00Z
- Sandbox: /tmp/tessariq-manual-test-TASK-026/

## Test Steps

### MT-001: --mount-agent-config flag exists with default false

- Severity: critical
- Mode: sandbox
- Derived from: "A new --mount-agent-config boolean flag exists on tessariq run with default false."
- Setup: Build the CLI binary.
- Command: `go run ./cmd/tessariq run --help`
- Expected: Help output includes `--mount-agent-config` flag description. Default is false.

### MT-002: DefaultConfig has MountAgentConfig false

- Severity: critical
- Mode: sandbox
- Derived from: "A new --mount-agent-config boolean flag exists on tessariq run with default false."
- Setup: Write a standalone Go program that calls run.DefaultConfig().
- Command: `go run ./cmd/manual-test-026/main.go`
- Expected: Output confirms MountAgentConfig is false.

### MT-003: DiscoverConfigDirs returns mounted for Claude Code with existing dir

- Severity: critical
- Mode: sandbox
- Derived from: "When the flag is set, Tessariq additionally mounts Claude Code ~/.claude/ to $HOME/.claude/"
- Setup: Create fake ~/.claude/ dir in sandbox.
- Command: `go run ./cmd/manual-test-026/main.go discover-claude-mounted`
- Expected: Status "mounted", 1 mount, container path /home/tessariq/.claude, ReadOnly true.

### MT-004: DiscoverConfigDirs returns mounted for OpenCode with existing dir

- Severity: critical
- Mode: sandbox
- Derived from: "When the flag is set, Tessariq additionally mounts OpenCode ~/.config/opencode/ to $HOME/.config/opencode/"
- Setup: Create fake ~/.config/opencode/ dir in sandbox.
- Command: `go run ./cmd/manual-test-026/main.go discover-opencode-mounted`
- Expected: Status "mounted", 1 mount, container path /home/tessariq/.config/opencode, ReadOnly true.

### MT-005: DiscoverConfigDirs returns missing_optional when dir absent

- Severity: critical
- Mode: sandbox
- Derived from: "Missing or unreadable optional config dirs do not fail the run"
- Setup: Use empty sandbox dir with no config dirs.
- Command: `go run ./cmd/manual-test-026/main.go discover-claude-missing`
- Expected: Status "missing_optional", 0 mounts, no error.

### MT-006: runtime.json records correct agent_config_mount values

- Severity: critical
- Mode: sandbox
- Derived from: "runtime.json records agent_config_mount as disabled or enabled and agent_config_mount_status as exactly one of disabled, mounted, missing_optional, or unreadable_optional"
- Setup: Create RuntimeInfo with each status combination.
- Command: `go run ./cmd/manual-test-026/main.go runtime-json`
- Expected: JSON output contains correct field values for all four status combinations.

### MT-007: Config dir mounts do not expose host HOME

- Severity: critical
- Mode: sandbox
- Derived from: "Tessariq does not mount arbitrary host-home paths as a side effect of the flag."
- Setup: Discover config dirs for both agents.
- Command: `go run ./cmd/manual-test-026/main.go no-home-exposure`
- Expected: No mount has ContainerPath equal to host home directory.

### MT-008: CLAUDE_CONFIG_DIR env var set when Claude Code config mounted

- Severity: major
- Mode: sandbox
- Derived from: Spec: "when Claude Code config directories are mounted, Tessariq MUST set CLAUDE_CONFIG_DIR=$HOME/.claude"
- Setup: Discover Claude Code config dirs with dir present.
- Command: `go run ./cmd/manual-test-026/main.go claude-env-var`
- Expected: EnvVars contains CLAUDE_CONFIG_DIR=/home/tessariq/.claude.
