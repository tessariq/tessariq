# Manual Test Plan

- Task: TASK-025-opencode-agent-runtime-integration
- Generated: 2026-03-31T09:09:27Z
- Sandbox: /tmp/tessariq-manual-test-TASK-025/ (sandbox mode)
  AND: Testcontainers RunEnv (container mode)

## Test Steps

### MT-001: agent.json records agent=opencode with requested/applied semantics

- Severity: critical
- Mode: sandbox
- Derived from: "agent.json records agent=opencode and the requested/applied option semantics required by the active spec."
- Setup: Call adapter factory with opencode agent and verify AgentInfo fields.
- Command: `go run ./cmd/manual-test-025/main.go`
- Expected: agent.json has schema_version=1, agent=opencode, requested has interactive=false, applied has interactive=false and model=false when model is set.

### MT-002: OpenCode integrates cleanly with run lifecycle (container e2e)

- Severity: critical
- Mode: container
- Derived from: "OpenCode integrates cleanly with the run lifecycle."
- Setup: Use StartRunEnvForBinary with opencode binary, set up auth with provider info, run tessariq run --agent opencode.
- Command: `go test -tags=manual_test ./cmd/tessariq/ -run TestManual_OpenCodeRunLifecycle -v -count=1`
- Expected: Exit code 0, output contains run_id, evidence_path, agent.json and runtime.json exist and are valid.

### MT-003: Missing opencode binary gives actionable error

- Severity: critical
- Mode: sandbox
- Derived from: "Tessariq validates that the opencode binary is present" and "Missing-OpenCode-binary failures identify the missing opencode binary"
- Setup: Call opencode.Process.Start with PATH pointing to empty dir.
- Command: `go run ./cmd/manual-test-025-binary/main.go`
- Expected: Error wraps exec.ErrNotFound, message contains "adapter binary", "opencode", "container image", "--image".

### MT-004: Auth mount contract with auth.json

- Severity: critical
- Mode: sandbox
- Derived from: "OpenCode works with the supported read-only auth-mount contract using ~/.local/share/opencode/auth.json."
- Setup: Create temp dir with auth.json, call authmount.Discover for opencode.
- Command: `go run ./cmd/manual-test-025-auth/main.go`
- Expected: Result has 1 mount, read-only, correct host/container paths.

### MT-005: Provider-aware egress profile with resolved provider

- Severity: critical
- Mode: sandbox
- Derived from: "OpenCode uses the provider-aware --egress auto profile: models.dev:443, the resolved provider base-URL host on 443, and opencode.ai:443 only when the resolved provider or auth flow requires it."
- Setup: Call ResolveProvider with various auth/config combos, then OpenCodeEndpoints.
- Command: `go run ./cmd/manual-test-025-egress/main.go`
- Expected: Non-OC-hosted returns [models.dev:443, provider:443]; OC-hosted returns [models.dev:443, provider:443, opencode.ai:443].

### MT-006: Unresolvable provider fails before container start

- Severity: critical
- Mode: container
- Derived from: "When the OpenCode provider host cannot be resolved from available config and auth state under --egress auto, Tessariq fails before container start with actionable guidance."
- Setup: Auth.json without provider field, run tessariq run --agent opencode.
- Command: `go test -tags=manual_test ./cmd/tessariq/ -run TestManual_OpenCodeUnresolvableProvider -v -count=1`
- Expected: Non-zero exit, output contains "configure the provider" and "--egress-allow".

### MT-007: opencode.ai:443 included only for OpenCode-hosted provider

- Severity: major
- Mode: sandbox
- Derived from: "opencode.ai:443 only when the resolved provider or auth flow requires it"
- Setup: Test OpenCodeEndpoints with isOpenCodeHosted=true and false.
- Command: `go run ./cmd/manual-test-025-conditional/main.go`
- Expected: opencode.ai present only when isOpenCodeHosted=true.
