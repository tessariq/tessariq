---
task: TASK-018-replace-yolo-with-interactive-and-cli-polish
timestamp: "2026-03-29T20:17:52Z"
agent: claude-opus-4-6
---

# Manual Test Plan: TASK-018

## Test Cases

### TC-1: --yolo and --egress-allow-reset removed
- Run `go run ./cmd/tessariq run --help`
- Verify `--yolo` does NOT appear in output
- Verify `--egress-allow-reset` does NOT appear in output

### TC-2: --interactive flag present with correct defaults
- Run `go run ./cmd/tessariq run --help`
- Verify `--interactive` appears with help text: "require human approval for agent tool use (use with --attach)"
- Verify no default value shown (boolean default false is not displayed by cobra)

### TC-3: --egress-no-defaults flag present with correct help
- Run `go run ./cmd/tessariq run --help`
- Verify `--egress-no-defaults` appears with help text: "ignore default allowlists; only --egress-allow entries apply"

### TC-4: Duration display polish
- Run `go run ./cmd/tessariq run --help`
- Verify `--timeout` shows `(default 30m)` not `(default 30m0s)`
- Verify `--grace` shows `(default 30s)` (already clean, but confirm format consistency)

### TC-5: --interactive without --attach warning
- Verify that the warning logic exists in `cmd/tessariq/run.go`
- The warning prints to stderr: "warning: --interactive without --attach; agent will block waiting for approval with no terminal attached"
- Note: full E2E testing of this requires a valid task file and clean repo; verified by code inspection

### TC-6: DefaultConfig returns correct field names
- Unit test `TestDefaultConfig` verifies `Interactive: false` and `EgressNoDefaults: false`
- Run `go test ./internal/run/ -run TestDefaultConfig -v`
