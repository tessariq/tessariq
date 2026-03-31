# Manual Test Plan

- Task: TASK-023-supported-agent-auth-mounts
- Generated: 2026-03-31T08:00:00Z
- Sandbox: /tmp/tessariq-manual-test-TASK-023/

## Test Steps

### MT-001: Claude Code Linux auth path knowledge

- Severity: critical
- Mode: sandbox
- Derived from: "Claude Code required auth paths are exactly: Linux: ~/.claude/.credentials.json and ~/.claude.json"
- Setup: Create sandbox with fake home containing both auth files.
- Command: `go run cmd/manual-test-023/main.go claude-code linux present`
- Expected: Discover returns 2 mounts with host paths under the fake home matching the documented paths.

### MT-002: Claude Code macOS auth path knowledge

- Severity: critical
- Mode: sandbox
- Derived from: "Claude Code required auth paths are exactly: macOS: ~/.claude/.credentials.json when a file-backed credential mirror exists, and ~/.claude.json"
- Setup: Create sandbox with fake home containing both auth files.
- Command: `go run cmd/manual-test-023/main.go claude-code darwin present`
- Expected: Discover returns 2 mounts for macOS with correct paths.

### MT-003: OpenCode auth path knowledge

- Severity: critical
- Mode: sandbox
- Derived from: "OpenCode required auth paths are exactly ~/.local/share/opencode/auth.json on Linux and macOS"
- Setup: Create sandbox with fake home containing opencode auth file.
- Command: `go run cmd/manual-test-023/main.go opencode linux present`
- Expected: Discover returns 1 mount for the opencode auth.json path.

### MT-004: Auto-detection before agent start

- Severity: critical
- Mode: sandbox
- Derived from: "Tessariq auto-detects the required supported-agent auth files or directories before agent start"
- Setup: Use Discover with fileExists that checks real filesystem fixtures.
- Command: `go run cmd/manual-test-023/main.go claude-code linux present` (with real files)
- Expected: Discover succeeds when files exist.

### MT-005: Deterministic in-container mount destinations

- Severity: critical
- Mode: sandbox
- Derived from: "Required auth files are mounted read-only into deterministic in-container locations"
- Setup: None (validates container paths from Discover output).
- Command: `go run cmd/manual-test-023/main.go check-container-paths`
- Expected: Claude Code container paths are /home/tessariq/.claude/.credentials.json and /home/tessariq/.claude.json; OpenCode is /home/tessariq/.local/share/opencode/auth.json.

### MT-006: No host HOME exposure

- Severity: critical
- Mode: sandbox
- Derived from: "Tessariq does not expose the host HOME directory inside the container"
- Setup: None (validates MountSpec output).
- Command: `go run cmd/manual-test-023/main.go check-no-home-exposure`
- Expected: No MountSpec has HostPath equal to homeDir or ContainerPath equal to homeDir.

### MT-007: Missing auth fails cleanly

- Severity: critical
- Mode: sandbox
- Derived from: "Tessariq fails cleanly when required supported-agent auth state is missing"
- Setup: Invoke Discover with no auth files present.
- Command: `go run cmd/manual-test-023/main.go claude-code linux missing`
- Expected: Returns AuthMissingError with actionable message.

### MT-008: macOS Keychain-only fails cleanly

- Severity: critical
- Mode: sandbox
- Derived from: "Tessariq fails cleanly for Claude Code on macOS when only Keychain-backed auth exists"
- Setup: Invoke Discover on macOS with claude.json present but credentials.json missing.
- Command: `go run cmd/manual-test-023/main.go claude-code darwin keychain-only`
- Expected: Returns KeychainOnlyError with message about file-backed setup.

### MT-009: Writable auth required fails cleanly

- Severity: major
- Mode: sandbox
- Derived from: "Tessariq fails cleanly when the selected agent requires writable auth refresh"
- Setup: WritableAuthRequiredError type exists and produces correct message.
- Command: `go run cmd/manual-test-023/main.go check-writable-error`
- Expected: Error message includes "read-only" and "pre-authenticated".

### MT-010: runtime.json records auth mount policy without secrets

- Severity: critical
- Mode: sandbox
- Derived from: "runtime.json records the read-only auth mount policy without recording secrets or host-home paths"
- Setup: Create RuntimeInfo with auth_mount_count=2.
- Command: `go run cmd/manual-test-023/main.go check-runtime-json`
- Expected: JSON contains auth_mount_mode "read-only", auth_mount_count 2, and no host path or secret fields.

### MT-011: No macOS Keychain reuse attempted

- Severity: major
- Mode: sandbox
- Derived from: "Tessariq does not attempt direct macOS Keychain reuse for Claude Code in v0.1.0"
- Setup: Verify code does not reference security CLI or Keychain APIs.
- Command: `grep -r 'security find-generic-password\|Keychain' internal/authmount/`
- Expected: No matches found.
