# Manual Test Plan

- Task: TASK-005-runner-bootstrap-timeout-and-status-lifecycle
- Generated: 2026-03-29T20:06:16Z
- Sandbox: /tmp/tessariq-manual-test-TASK-005/

## Test Steps

### MT-001: status.json exists even on bootstrap failure path

- Severity: critical
- Derived from: "status.json exists even on bootstrap failure and is created before long-running runner work begins."
- Setup: Create a temp git repo with a task file, run the CLI to produce evidence, then verify status.json exists.
- Command: See execution steps below (multi-step).
- Expected: status.json file exists in the evidence directory with valid JSON.

### MT-002: Runner lifecycle produces all five terminal states with timestamps

- Severity: critical
- Derived from: "Runner lifecycle produces exactly the v0.1.0 terminal states success, failed, timeout, killed, or interrupted, with valid started_at and finished_at timestamps."
- Setup: Run unit tests that cover all five terminal state paths.
- Command: `go test ./internal/runner/ -run 'TestRunner_SuccessPath|TestRunner_FailedProcess|TestRunner_TimeoutPath|TestNewTerminalStatus_AllStates' -v -count=1`
- Expected: All tests pass, confirming success, failed, timeout, killed, interrupted states with valid timestamps.

### MT-003: Timeout writes timed_out and exit_code before escalation

- Severity: critical
- Derived from: "Timeout handling writes the expected evidence, including timed_out and exit_code, before escalation."
- Setup: Run the timeout integration test.
- Command: `go test -tags=integration ./internal/runner/ -run 'TestRunnerIntegration_TimeoutWritesFlag' -v -count=1`
- Expected: Test passes. status.json contains timed_out=true and timeout.flag exists.

### MT-004: status.json has all minimum required fields

- Severity: critical
- Derived from: "status.json includes the minimum required fields for schema_version, state, started_at, finished_at, exit_code, and timed_out."
- Setup: Run the CLI and inspect the produced status.json.
- Command: See execution steps below (multi-step).
- Expected: status.json contains exactly schema_version, state, started_at, finished_at, exit_code, timed_out.

### MT-005: container_name recorded in manifest.json

- Severity: critical
- Derived from: "Runner bootstrap records the deterministic container name tessariq-<run_id> in manifest.json before detached guidance prints it."
- Setup: Run the CLI and inspect manifest.json.
- Command: See execution steps below (multi-step).
- Expected: manifest.json contains container_name field matching tessariq-<run_id>.

### MT-006: run.log and runner.log durable on failure paths

- Severity: major
- Derived from: "run.log and runner.log remain durable even when bootstrap or timeout paths fail."
- Setup: Run integration tests for log durability.
- Command: `go test -tags=integration ./internal/runner/ -run 'TestRunnerIntegration_EvidenceDurability' -v -count=1`
- Expected: Test passes. Both run.log and runner.log exist after a failed process.
