# Manual Test Report

- Task: TASK-012-proxy-topology-and-egress-artifacts
- Executed: 2026-03-31T19:33:00Z
- Verdict: pass

## Results

### MT-001: egress.compiled.yaml schema
- Status: pass
- Observation: File written with schema_version: 1, allowlist_source: built_in, destinations with host+port.

### MT-002: egress.events.jsonl format
- Status: pass
- Observation: Squid TCP_DENIED entries parsed, round-trip write+read preserves all fields.

### MT-003: squid config CONNECT tunneling
- Status: pass
- Observation: Generated config contains CONNECT method ACL, SSL_ports, allow CONNECT rule, deny all.

### MT-004: allowlist provenance in compiled YAML
- Status: pass
- Observation: allowlist_source "cli" and exact destinations preserved in round-trip.

### MT-005: Claude Code built-in endpoints
- Status: pass
- Observation: api.anthropic.com, claude.ai, platform.claude.com all present alongside baseline endpoints.

### MT-006: OpenCode provider-aware endpoints
- Status: pass
- Observation: models.dev:443 and resolved provider host api.openai.com:443 present.

### MT-007: blocked destination UX
- Status: pass
- Observation: Events written and read back correctly with host, port, reason fields for UX display.

### MT-008: container network flag
- Status: pass
- Observation: NetworkName field on container.Config accepted; --net emission verified by unit test.

### MT-009: full proxy lifecycle (integration)
- Status: pass
- Observation: TopologySetupAndTeardown passed in 1.50s. Network/container created and cleaned up, evidence files written.

### MT-010: proxy enforcement allows and blocks (integration)
- Status: pass
- Observation: ProxyAllowsAndBlocks passed in 2.83s. Allowed dest succeeded via CONNECT, blocked dest denied, events JSONL recorded blocked entry.

### MT-011: e2e proxy mode writes egress evidence
- Status: pass
- Observation: ProxyModeWritesEgressEvidence passed in 17.0s. egress.compiled.yaml and egress.events.jsonl exist with correct schema and 0600 permissions.

## Summary

- Total: 11 | Pass: 11 | Fixed: 0 | Failed: 0 | Skipped: 0
