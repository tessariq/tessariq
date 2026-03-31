# Manual Test Plan

- Task: TASK-012-proxy-topology-and-egress-artifacts
- Generated: 2026-03-31T19:26:37Z
- Testcontainers RunEnv (container mode) and sandbox mode

## Test Steps

### MT-001: Proxy mode emits egress.compiled.yaml with correct schema

- Severity: critical
- Mode: sandbox
- Derived from: "`egress.compiled.yaml` is emitted in proxy mode with `schema_version`, `allowlist_source`, and fully resolved destination `host` and `port` entries."
- Setup: Call NewCompiledAllowlist + WriteCompiledYAML with built_in source and sample destinations.
- Command: `go run ./cmd/manual-test-012/main.go`
- Expected: File exists, YAML contains schema_version: 1, allowlist_source: built_in, destinations with host+port fields.

### MT-002: Proxy mode emits egress.events.jsonl for blocked attempts

- Severity: critical
- Mode: sandbox
- Derived from: "`egress.events.jsonl` is emitted only in proxy mode and records blocked attempts alongside the resolved allowlist context."
- Setup: Parse a sample Squid access log with TCP_DENIED entries, write events JSONL.
- Command: `go run ./cmd/manual-test-012/main.go`
- Expected: File exists, each line is valid JSON with timestamp, host, port, action=blocked, reason, squid_result.

### MT-003: Squid config supports CONNECT tunneling for HTTPS

- Severity: critical
- Mode: sandbox
- Derived from: "HTTPS and WSS CONNECT-style traffic is supported through the allowlisted proxy path."
- Setup: Generate squid conf for destinations on port 443.
- Command: `go run ./cmd/manual-test-012/main.go`
- Expected: Config contains `acl CONNECT method CONNECT`, `http_access allow CONNECT SSL_ports allowed_dest`, `acl SSL_ports port 443`.

### MT-004: Compiled YAML records allowlist provenance without re-derivation

- Severity: major
- Mode: sandbox
- Derived from: "Proxy evidence records both allowlist provenance and the fully resolved destinations without re-derivation."
- Setup: Create compiled allowlist with "cli" source and specific destinations.
- Command: `go run ./cmd/manual-test-012/main.go`
- Expected: allowlist_source is "cli", destinations list matches input exactly.

### MT-005: Claude Code built-in endpoints appear in compiled allowlist under --egress auto

- Severity: critical
- Mode: sandbox
- Derived from: "The proxy topology works with Claude Code's fixed first-party endpoints under `--egress auto`."
- Setup: Call ClaudeCodeEndpoints() + BaselineEndpoints(), build compiled allowlist.
- Command: `go run ./cmd/manual-test-012/main.go`
- Expected: Destinations include api.anthropic.com:443, claude.ai:443, platform.claude.com:443 plus baseline hosts.

### MT-006: OpenCode provider-aware endpoints in compiled allowlist

- Severity: critical
- Mode: sandbox
- Derived from: "The proxy topology works with OpenCode's provider-aware endpoint profile under `--egress auto`, including `models.dev:443` and the resolved provider host on `443`."
- Setup: Call OpenCodeEndpoints with a resolved provider host, build compiled allowlist.
- Command: `go run ./cmd/manual-test-012/main.go`
- Expected: Destinations include models.dev:443 and the resolved provider host on 443.

### MT-007: Blocked-destination UX prints actionable guidance

- Severity: major
- Mode: sandbox
- Derived from: "Blocked-destination failures tell the user which `host:port` was blocked and how to allow it through user config or CLI flags, or rerun with explicit open egress."
- Setup: Write events JSONL with blocked entries, call printBlockedDestinations.
- Command: `go run ./cmd/manual-test-012/main.go`
- Expected: Output mentions the blocked host:port, --egress-allow, config.yaml, --unsafe-egress.

### MT-008: Container network flag emitted in docker create args

- Severity: major
- Mode: sandbox
- Derived from: "Proxy mode integrates with the runner/container lifecycle and enforces host:port allowlists."
- Setup: Create container.Config with NetworkName, verify buildCreateArgs output.
- Command: `go run ./cmd/manual-test-012/main.go`
- Expected: Args contain --net followed by the network name.

### MT-009: Full proxy lifecycle via integration topology test

- Severity: critical
- Mode: container
- Derived from: "Proxy mode integrates with the runner/container lifecycle and enforces host:port allowlists with default port `443`."
- Setup: Run existing integration test that exercises full Setup+Teardown.
- Command: `go test -tags=integration -v -count=1 -run TestIntegration_TopologySetupAndTeardown ./internal/proxy/...`
- Expected: Test passes, network and container created/removed, evidence files written.

### MT-010: Proxy enforcement allows and blocks correctly

- Severity: critical
- Mode: container
- Derived from: "HTTPS and WSS CONNECT-style traffic is supported through the allowlisted proxy path" + blocked events recorded.
- Setup: Run existing integration test that verifies allow/block behavior.
- Command: `go test -tags=integration -v -count=1 -run TestIntegration_ProxyAllowsAndBlocks ./internal/proxy/...`
- Expected: Allowed destination succeeds, blocked destination fails, events JSONL contains blocked entry.

### MT-011: E2E proxy mode writes egress evidence artifacts

- Severity: critical
- Mode: container
- Derived from: Full end-to-end verification of proxy evidence in a real CLI run.
- Setup: Run existing e2e test.
- Command: `go test -tags=e2e -v -count=1 -run TestE2E_ProxyModeWritesEgressEvidence ./cmd/tessariq/...`
- Expected: egress.compiled.yaml and egress.events.jsonl exist with correct schema and 0600 permissions.
