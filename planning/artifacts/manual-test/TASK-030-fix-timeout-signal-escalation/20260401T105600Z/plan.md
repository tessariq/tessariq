# Manual Test Plan

- Task: TASK-030-fix-timeout-signal-escalation
- Generated: 2026-04-01T10:56:00Z
- Sandbox: /tmp/tessariq-manual-test-TASK-030/

## Test Steps

### MT-001: SIGTERM sent before SIGKILL on timeout (detached)

- Severity: critical
- Mode: sandbox
- Derived from: "On timeout, the runner sends SIGTERM (via docker stop) to the container first, not SIGKILL."
- Setup: Create a sandbox dir, build a Go test program that uses signalRecordingProcess (ignores SIGTERM) with short timeout/grace.
- Command: `go run ./cmd/manual-test-030/main.go -test mt001`
- Expected: Signals recorded are [SIGTERM, SIGKILL] in order. Exit code 0.

### MT-002: No SIGKILL when process exits after SIGTERM

- Severity: critical
- Mode: sandbox
- Derived from: "After the configured grace period expires without the container exiting, the runner escalates to SIGKILL."
- Setup: Use signalRecordingProcess that exits promptly on SIGTERM.
- Command: `go run ./cmd/manual-test-030/main.go -test mt002`
- Expected: Only SIGTERM recorded, no SIGKILL. Exit code 0.

### MT-003: timeout.flag written before first signal

- Severity: critical
- Mode: sandbox
- Derived from: "timeout.flag is written before the first signal (SIGTERM), not after."
- Setup: Use signalRecordingProcess with onSignal hook that checks for timeout.flag at SIGTERM time.
- Command: `go run ./cmd/manual-test-030/main.go -test mt003`
- Expected: timeout.flag exists when first signal is sent. Exit code 0.

### MT-004: Terminal state is timeout for both escalation paths

- Severity: major
- Mode: sandbox
- Derived from: "The terminal state for a timed-out run remains timeout regardless of which signal ultimately stops the container."
- Setup: Run two scenarios: one that exits on SIGTERM, one that requires SIGKILL. Both must produce StateTimeout.
- Command: `go run ./cmd/manual-test-030/main.go -test mt004`
- Expected: Both runs produce state "timeout". Exit code 0.

### MT-005: timed_out field is true for both paths

- Severity: major
- Mode: sandbox
- Derived from: "The status.json timed_out field is true for both graceful and forced timeout exits."
- Setup: Run two scenarios (graceful SIGTERM exit, forced SIGKILL). Read status.json from both.
- Command: `go run ./cmd/manual-test-030/main.go -test mt005`
- Expected: timed_out is true in both status.json files. Exit code 0.
