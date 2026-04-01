# Manual Test Plan

- Task: TASK-037-prestart-agent-binary-validation
- Generated: 2026-04-01T10:59:00Z
- Testcontainers RunEnv (container mode)

## Test Steps

### MT-001: Missing claude binary detected before agent start

- Severity: critical
- Mode: container
- Derived from: "Before starting the agent command, Tessariq validates that the selected agent binary exists in the resolved runtime image."
- Setup: Build tessariq binary, create a RunEnv with Docker socket, build a bare Alpine image WITHOUT the claude binary, set up auth + git repo.
- Command: Run `tessariq run --image <bare-image> --egress none tasks/sample.md` inside the RunEnv container.
- Expected: Non-zero exit, output contains `"claude"` (binary name), `claude-code` (agent), `--image` (guidance). No `run_id:` in output (failed before agent start).

### MT-002: Missing opencode binary detected before agent start

- Severity: critical
- Mode: container
- Derived from: "Validation behavior is implemented for both supported agents (claude-code, opencode)."
- Setup: Same as MT-001 but with opencode auth and `--agent opencode`.
- Command: Run `tessariq run --agent opencode --image <bare-image> --egress none tasks/sample.md` inside the RunEnv container.
- Expected: Non-zero exit, output contains `"opencode"` (binary name), `opencode` (agent), `--image` (guidance). No `run_id:` in output.

### MT-003: Error message includes all required fields per spec

- Severity: major
- Mode: container
- Derived from: "Missing binary failures occur before agent start and include: missing binary name, selected agent, and guidance to use a compatible runtime image or --image override."
- Setup: Reuse MT-001 setup.
- Command: Same as MT-001 but capture full error text.
- Expected: Error text contains the quoted binary name, the agent identifier, and the phrase "--image" for override guidance.

### MT-004: Successful run path unchanged with valid image

- Severity: critical
- Mode: container
- Derived from: "Existing successful run path for valid images remains unchanged."
- Setup: Build tessariq binary, create a RunEnv with a working fake claude binary image (standard setupRunEnv pattern).
- Command: Run `tessariq run --image <valid-image> --egress none tasks/sample.md` inside the RunEnv container.
- Expected: Exit code 0, output contains `run_id:`, `evidence_path:`, `attach:`, `promote:`.
