# Manual Test Plan

- Task: TASK-031-pin-repair-container-image
- Generated: 2026-04-01T10:54:50Z
- Sandbox: /tmp/tessariq-manual-test-TASK-031/

## Test Steps

### MT-001: Repair image pinned by digest

- Severity: critical
- Mode: sandbox
- Derived from: "The repair container image is pinned by digest (e.g., alpine@sha256:<digest>) rather than a mutable tag."
- Setup: none
- Command: `grep -c '@sha256:' internal/workspace/provision.go`
- Expected: exit 0, output is "1" (exactly one digest-pinned reference)

### MT-002: No mutable tag remains in production code

- Severity: critical
- Mode: sandbox
- Derived from: "The repair container image is pinned by digest rather than a mutable tag."
- Setup: none
- Command: `grep -c 'alpine:latest' internal/workspace/provision.go`
- Expected: exit 1 (no matches), confirming no mutable tag

### MT-003: Pinned digest in well-named constant

- Severity: major
- Mode: sandbox
- Derived from: "The pinned digest is documented in a constant or config that is easy to update during maintenance."
- Setup: Write a Go program that imports the workspace package and verifies the constant exists and contains a digest.
- Command: `go run cmd/manual-test-031/main.go`
- Expected: exit 0, output confirms constant name and digest format

### MT-004: Only worktree path mounted

- Severity: critical
- Mode: sandbox
- Derived from: "The repair container only mounts the disposable worktree path (no evidence, auth, or config mounts)."
- Setup: Write a Go program that calls buildRepairArgs and verifies mount count.
- Command: `go run cmd/manual-test-031/main.go`
- Expected: exit 0, output confirms exactly one -v mount pointing to worktree:/work

### MT-005: Repair runs as root

- Severity: major
- Mode: sandbox
- Derived from: "Repair continues to run as root inside the container (required for chown)."
- Setup: Same Go program verifies --user root in args.
- Command: `go run cmd/manual-test-031/main.go`
- Expected: exit 0, output confirms --user root present

### MT-006: Failed image pull produces actionable error

- Severity: major
- Mode: sandbox
- Derived from: "A failed image pull produces an actionable error message."
- Setup: Inspect the error format string in repairWorkspaceOwnership.
- Command: `grep -n 'repair workspace ownership' internal/workspace/provision.go`
- Expected: Error wraps docker output with workspace path context, providing actionable information.
