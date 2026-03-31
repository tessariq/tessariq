# Manual Test Report

- Task: TASK-025-opencode-agent-runtime-integration
- Executed: 2026-03-31T09:09:27Z
- Verdict: pass

## Results

### MT-001: agent.json records agent=opencode with requested/applied semantics

- Status: pass
- Observation: schema_version=1, agent=opencode, requested has interactive=false and model=gpt-5.4, applied has interactive=false and model=false. All 6 checks passed.

### MT-002: OpenCode integrates cleanly with run lifecycle (container e2e)

- Status: pass
- Observation: `tessariq run --agent opencode` exits 0, output contains run_id and evidence_path. agent.json has agent=opencode with applied.interactive=false. runtime.json has schema_version=1 and auth_mount_mode=read-only.

### MT-003: Missing opencode binary gives actionable error

- Status: pass
- Observation: Error wraps exec.ErrNotFound, message contains "adapter binary", "opencode", "container image", and "--image". All 5 checks passed.

### MT-004: Auth mount contract with auth.json

- Status: pass
- Observation: Discover returns agent=opencode, 1 read-only mount with correct host/container paths ending in auth.json. All 5 checks passed.

### MT-005: Provider-aware egress profile with resolved provider

- Status: pass
- Observation: Non-OC-hosted returns 2 endpoints (models.dev:443, provider:443). OC-hosted returns 3 endpoints (adds opencode.ai:443). Auth fallback works. All port 443. All 9 checks passed.

### MT-006: Unresolvable provider fails before container start

- Status: pass
- Observation: Auth.json without provider info causes non-zero exit before container start. Output contains "configure the provider" and "--egress-allow" guidance.

### MT-007: opencode.ai:443 included only for OpenCode-hosted provider

- Status: pass
- Observation: opencode.ai absent when includeOpenCodeAI=false, present when true. Both checks passed.

## Summary

- Total: 7 | Pass: 7 | Fixed: 0 | Failed: 0 | Skipped: 0
