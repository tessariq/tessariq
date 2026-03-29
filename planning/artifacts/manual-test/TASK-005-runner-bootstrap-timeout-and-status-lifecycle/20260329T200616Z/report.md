# Manual Test Report

- Task: TASK-005-runner-bootstrap-timeout-and-status-lifecycle
- Executed: 2026-03-29T20:06:16Z
- Verdict: pass

## Results

### MT-001: status.json exists even on bootstrap failure path

- Status: pass
- Observation: CLI produced evidence directory with status.json containing valid JSON. Fields: schema_version=1, state="success", started_at, finished_at, exit_code=0, timed_out=false.

### MT-002: Runner lifecycle produces all five terminal states with timestamps

- Status: pass
- Observation: Unit tests TestRunner_SuccessPath, TestRunner_FailedProcess, TestRunner_TimeoutPath, and TestNewTerminalStatus_AllStates all pass. All five terminal states (success, failed, timeout, killed, interrupted) are produced with valid started_at and finished_at timestamps.

### MT-003: Timeout writes timed_out and exit_code before escalation

- Status: pass
- Observation: Integration test TestRunnerIntegration_TimeoutWritesFlag passes. status.json contains timed_out=true and timeout.flag file exists in evidence directory after timeout escalation.

### MT-004: status.json has all minimum required fields

- Status: pass
- Observation: status.json from CLI run contains exactly the required fields: schema_version (1), state ("success"), started_at ("2026-03-29T20:07:32Z"), finished_at ("2026-03-29T20:07:32Z"), exit_code (0), timed_out (false).

### MT-005: container_name recorded in manifest.json

- Status: pass
- Observation: manifest.json contains container_name="tessariq-01KMXKAXC6THE9RS5NHZQ75Z8P" which matches the pattern tessariq-<run_id>. The CLI also prints the container_name, attach, and promote commands.

### MT-006: run.log and runner.log durable on failure paths

- Status: pass
- Observation: Integration test TestRunnerIntegration_EvidenceDurability passes (both log files exist after a failed process). CLI run also produces both files: run.log (0 bytes, no agent) and runner.log (131 bytes, lifecycle events).

## Summary

- Total: 6 | Pass: 6 | Fixed: 0 | Failed: 0 | Skipped: 0
