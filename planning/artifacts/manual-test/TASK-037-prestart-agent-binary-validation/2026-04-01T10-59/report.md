# Manual Test Report

- Task: TASK-037-prestart-agent-binary-validation
- Executed: 2026-04-01T11:09:00Z
- Verdict: pass

## Results

### MT-001: Missing claude binary detected before agent start

- Status: pass
- Observation: Exit code 1. Output: `agent claude-code: binary "claude" not found in runtime image tessariq-manual-no-claude; use a compatible runtime image or specify --image to override`. No `run_id:` in output — failed before agent start as expected.

### MT-002: Missing opencode binary detected before agent start

- Status: pass
- Observation: Exit code 1. Output: `agent opencode: binary "opencode" not found in runtime image tessariq-manual-no-opencode; use a compatible runtime image or specify --image to override`. No `run_id:` in output — failed before agent start as expected.

### MT-003: Error message includes all required fields per spec

- Status: pass
- Observation: Error text contains: quoted binary name (`"claude"`), agent identifier (`claude-code`), `compatible runtime image` phrase, `--image` override guidance. All four spec-required fields are present.

### MT-004: Successful run path unchanged with valid image

- Status: pass
- Observation: Exit code 0. Output contains `run_id:`, `evidence_path:`, `attach: tessariq attach`, `promote: tessariq promote`. The successful run path is unaffected by the new validation.

## Summary

- Total: 4 | Pass: 4 | Fixed: 0 | Failed: 0 | Skipped: 0
