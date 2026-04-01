# Manual Test Report

- Task: TASK-031-pin-repair-container-image
- Executed: 2026-04-01T10:55:30Z
- Verdict: pass

## Results

### MT-001: Repair image pinned by digest

- Status: pass
- Observation: `grep -c '@sha256:' internal/workspace/provision.go` returned "1" — exactly one digest-pinned reference exists.

### MT-002: No mutable tag remains in production code

- Status: pass
- Observation: `grep -c 'alpine:latest' internal/workspace/provision.go` returned "0" with exit 1 — no mutable tags remain.

### MT-003: Pinned digest in well-named constant

- Status: pass
- Observation: AST parse confirmed `repairImage` constant exists with value `"alpine@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659"`.

### MT-004: Only worktree path mounted

- Status: pass
- Observation: `buildRepairArgs` contains exactly one `-v` flag targeting `:/work`. No evidence, auth, or config mounts present.

### MT-005: Repair runs as root

- Status: pass
- Observation: `buildRepairArgs` contains `"--user", "root"` arguments.

### MT-006: Failed image pull produces actionable error

- Status: pass
- Observation: Error format at provision.go:93 wraps workspace path, Docker combined output (includes pull failure details and image reference), and underlying error — provides actionable context for diagnosis.

## Summary

- Total: 6 | Pass: 6 | Fixed: 0 | Failed: 0 | Skipped: 0
