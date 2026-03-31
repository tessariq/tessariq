# Manual Test Report

- Task: TASK-023-supported-agent-auth-mounts
- Executed: 2026-03-31T08:03:00Z
- Verdict: pass

## Results

### MT-001: Claude Code Linux auth path knowledge

- Status: pass
- Observation: Discover returned 2 mounts with correct host paths (credentials.json and .claude.json) and container paths under /home/tessariq/.

### MT-002: Claude Code macOS auth path knowledge

- Status: pass
- Observation: Discover returned 2 mounts for macOS with identical path structure as Linux.

### MT-003: OpenCode auth path knowledge

- Status: pass
- Observation: Discover returned 1 mount for ~/.local/share/opencode/auth.json with container path /home/tessariq/.local/share/opencode/auth.json.

### MT-004: Auto-detection before agent start

- Status: pass
- Observation: Discover uses injected fileExists function against real filesystem fixtures. Succeeds when files exist.

### MT-005: Deterministic in-container mount destinations

- Status: pass
- Observation: Claude Code: /home/tessariq/.claude/.credentials.json and /home/tessariq/.claude.json. OpenCode: /home/tessariq/.local/share/opencode/auth.json. All match spec.

### MT-006: No host HOME exposure

- Status: pass
- Observation: No MountSpec has HostPath or ContainerPath equal to the home directory.

### MT-007: Missing auth fails cleanly

- Status: pass
- Observation: AuthMissingError returned with message "supported auth files or directories for claude-code were not found; authenticate claude-code locally first".

### MT-008: macOS Keychain-only fails cleanly

- Status: pass
- Observation: KeychainOnlyError returned with message "v0.1.0 supports Claude Code auth reuse on macOS only when ~/.claude/.credentials.json exists; use a compatible file-backed setup".

### MT-009: Writable auth required fails cleanly

- Status: pass
- Observation: WritableAuthRequiredError produces message "v0.1.0 supports only read-only auth and config mounts; use a compatible pre-authenticated setup".

### MT-010: runtime.json records auth mount policy without secrets

- Status: pass
- Observation: runtime.json contains auth_mount_mode "read-only", auth_mount_count 2, and no forbidden fields (host_path, token, secret, credentials).

### MT-011: No macOS Keychain reuse attempted

- Status: pass
- Observation: grep for "security find-generic-password" and "Keychain" found only the Go error type name KeychainOnlyError — no macOS Keychain API calls.

## Summary

- Total: 11 | Pass: 11 | Fixed: 0 | Failed: 0 | Skipped: 0
