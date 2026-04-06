---
id: TASK-081-model-aware-opencode-egress
title: Include model provider host in OpenCode proxy allowlist
status: pending
priority: p1
depends_on:
    - TASK-079-forward-model-flag-and-adapter-interface
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#built-in-tessariq-allowlist-profile
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-06T09:00:00Z"
areas:
    - adapter
    - egress
    - cli
verification:
    unit:
        required: true
        commands:
            - go test ./internal/adapter/opencode/...
            - go test ./internal/adapter/...
            - go test ./cmd/tessariq/...
            - go test ./...
        rationale: Provider parsing, known-host lookup, and allowlist resolution are pure logic with conditional branching.
    integration:
        required: false
        commands: []
        rationale: No new subsystem boundaries; allowlist resolution is exercised through unit tests on resolveAllowlistCore.
    e2e:
        required: false
        commands: []
        rationale: Proxy topology is unchanged; the change is in which destinations feed into it.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: Known-provider map lookup, fallback logic, and model-prefix parsing all have mutation-sensitive branches.
    manual_test:
        required: false
        commands: []
        rationale: All behavior can be validated through unit tests on resolveAllowlistCore and provider functions.
---

## Summary

TASK-079 forwarded `--model` to the OpenCode CLI, but proxy allowlist resolution still derives egress destinations exclusively from auth/config provider files in `resolveAllowlistCore`. When `--model provider/model` selects a provider that differs from the configured one, the model provider's API host is absent from the allowlist. Under the default `--egress auto`/`proxy` path with no explicit `--egress-allow`, the run fails on egress denial while `agent.json` reports `applied.model=true`.

This task adds model-aware provider resolution: parse the provider prefix from `--model`, look it up in a curated known-providers map, and include the model provider's API host in the built-in allowlist alongside the configured provider's host. Unknown providers fail with clear `--egress-allow` guidance.

## Acceptance Criteria

- When `--model provider/model` specifies a known provider whose API host differs from the configured provider, the built-in allowlist includes both the configured and model provider hosts.
- When `--model provider/model` specifies the same provider as the configured one, the allowlist is unchanged (no duplicate endpoint).
- When `--model` has no provider prefix (no `/`), the allowlist uses only the configured provider (existing behavior).
- When `--model provider/model` specifies a provider prefix not in the known map, tessariq fails before container start with a clear error naming the provider and suggesting `--egress-allow` or `--egress open`.
- When the configured provider is unresolvable from auth/config but `--model` provides a known provider, the model provider is used as a fallback for allowlist construction (the run may still fail at auth discovery, but the allowlist error is no longer the first failure).
- When higher-precedence allowlist sources exist (CLI `--egress-allow` or user config), model provider resolution is skipped entirely (no error for unknown providers in that case).
- `ProviderUnresolvableError` message mentions `--model provider/model` as an alternative resolution path.
- Existing behavior for runs without `--model` and for Claude Code is unchanged.
- Spec amendment documents the `--model` provider signal in the built-in allowlist profile section and failure UX table.

## Known Provider Map

The map covers providers with stable, predictable single-host API endpoints. Providers requiring wildcard host patterns (Amazon Bedrock, Azure OpenAI, Google Vertex AI) are excluded and must use `--egress-allow`.

Verify exact provider ID prefixes against `models.dev/api.json` during implementation — OpenCode fetches this catalog at runtime and uses its provider IDs in the `provider/model` format.

### Tier 1 — Major AI providers

| Provider prefix | API host |
|-----------------|----------|
| `anthropic` | `api.anthropic.com` |
| `openai` | `api.openai.com` |
| `google` | `generativelanguage.googleapis.com` |
| `mistral` | `api.mistral.ai` |
| `deepseek` | `api.deepseek.com` |
| `xai` | `api.x.ai` |
| `cohere` | `api.cohere.com` |

### Tier 2 — Inference providers

| Provider prefix | API host |
|-----------------|----------|
| `groq` | `api.groq.com` |
| `fireworks` | `api.fireworks.ai` |
| `together` | `api.together.xyz` |
| `cerebras` | `api.cerebras.ai` |
| `deepinfra` | `api.deepinfra.com` |
| `perplexity` | `api.perplexity.ai` |
| `openrouter` | `openrouter.ai` |

### Tier 3 — Coding subscriptions and specialized

| Provider prefix | API host | Notes |
|-----------------|----------|-------|
| `opencode` | `opencode.ai` | Also triggers `isOpenCodeHosted` endpoint handling |
| `minimax` | `api.minimax.io` | International endpoint |
| `moonshot` | `api.moonshot.ai` | International endpoint |
| `zhipu` | `api.z.ai` | z.ai international endpoint; verify provider ID against models.dev |

### Tier 4 — Platform and hub providers

| Provider prefix | API host |
|-----------------|----------|
| `github-copilot` | `api.githubcopilot.com` |
| `github-models` | `models.github.ai` |
| `nvidia` | `integrate.api.nvidia.com` |
| `huggingface` | `router.huggingface.co` |
| `llama` | `api.llama.com` |
| `morph` | `api.morphllm.com` |
| `venice` | `api.venice.ai` |

### Excluded — Wildcard host patterns (require `--egress-allow`)

| Provider | Host pattern | Reason |
|----------|--------------|--------|
| Amazon Bedrock | `bedrock-runtime.{region}.amazonaws.com` | Region-specific subdomain |
| Azure OpenAI | `{resource}.openai.azure.com` | Resource-specific subdomain |
| Google Vertex AI | `{region}-aiplatform.googleapis.com` | Region-specific subdomain |

## Implementation Sketch

### 1. New functions in `internal/adapter/opencode/provider.go`

- `ParseModelProvider(model string) string` — extract provider prefix before `/`.
- `KnownProviderHost(provider string) (host string, ok bool)` — look up known API host.
- `ModelProviderUnknownError{Provider string}` — new error type.
- `IsOpenCodeHostedHost(host string) bool` — exported wrapper around existing unexported `isOpenCodeHosted`.
- Update `ProviderUnresolvableError` message to mention `--model provider/model`.

### 2. Model-aware resolution in `cmd/tessariq/run.go`

Extract `resolveOpenCodeEndpoints(model, homeDir string, deps resolveAllowlistDeps) ([]adapter.Destination, error)`:

1. Resolve configured provider from auth/config (existing logic).
2. If `model` has a provider prefix, look it up in the known map. Fail with `ModelProviderUnknownError` if unknown.
3. If configured provider resolution fails with `ProviderUnresolvableError` and model provider is known, use model provider as fallback.
4. If configured provider resolution fails with `os.ErrNotExist`, return `AuthMissingError` (unchanged).
5. Build endpoints: `OpenCodeEndpoints(configuredHost, isOpenCodeHosted)` + append model host if different.
6. `includeOpenCodeAI` is true if either host is OpenCode-hosted.

Replace the `case "opencode"` block in `resolveAllowlistCore` with a call to this helper.

### 3. Spec amendment in `specs/tessariq-v0.1.0.md`

Add to built-in allowlist profile section (~line 347-349):
- `--model provider/model` with a known provider prefix adds that provider's API host to the built-in endpoints.
- Unknown provider prefix with proxy egress fails before container start.

Add to failure UX table (~line 545):
- New row for unknown `--model` provider prefix.
- Amend existing provider-unresolvable row to include `--model` as alternative.

## Test Expectations

### `internal/adapter/opencode/provider_test.go`

- Table-driven tests for `ParseModelProvider`: with prefix, without prefix, empty string, slash-at-start, slash-at-end.
- Table-driven tests for `KnownProviderHost`: each known provider returns correct host, unknown returns `("", false)`.
- `ModelProviderUnknownError` message contains provider name and `--egress-allow`.
- `IsOpenCodeHostedHost` returns true for `opencode.ai` and `*.opencode.ai`, false for others.
- Updated `ProviderUnresolvableError` message mentions `--model`.

### `cmd/tessariq/run_test.go`

Add `model` field to `TestResolveAllowlistCore_OpenCode` test struct. New cases:

| Case | `--model` | Auth provider | Expected |
|------|-----------|---------------|----------|
| Different known provider adds endpoint | `openai/gpt-4o` | anthropic | `built_in`, destinations include `api.openai.com:443` |
| Same provider no extra endpoint | `anthropic/claude-sonnet-4` | anthropic | `built_in`, no extra host |
| Unknown provider errors | `mistral-custom/model` | anthropic | `ModelProviderUnknownError` |
| No prefix uses configured only | `claude-sonnet-4` | anthropic | `built_in`, no extra host |
| Known model fallback when config unresolvable | `openai/gpt-4o` | (no provider in auth) | `built_in`, uses openai host |
| CLI bypasses model resolution | `unknown-provider/model` | (any) | `cli` source, no error |
| User config bypasses model resolution | `unknown-provider/model` | (any) | `user_config` source, no error |
| No model present | (empty) | anthropic | existing behavior unchanged |

## TDD Plan

1. RED: add `TestParseModelProvider` — fails (function does not exist).
2. GREEN: implement `ParseModelProvider`.
3. RED: add `TestKnownProviderHost` — fails (function does not exist).
4. GREEN: implement known-providers map and `KnownProviderHost`.
5. RED: add `TestModelProviderUnknownError_Message` — fails (type does not exist).
6. GREEN: implement `ModelProviderUnknownError`.
7. RED: add `TestIsOpenCodeHostedHost` — fails (function does not exist).
8. GREEN: export `IsOpenCodeHostedHost`.
9. RED: add model-aware cases to `TestResolveAllowlistCore_OpenCode` — fails (model field not wired).
10. GREEN: extract `resolveOpenCodeEndpoints`, wire `cfg.Model`, implement model-provider merging.
11. REFACTOR: update `ProviderUnresolvableError` message, verify all existing tests still pass.

## Notes

- The known-providers map is OpenCode-specific. Claude Code always uses Anthropic, so `--model` for Claude Code does not affect egress.
- `models.dev:443` is already in the OpenCode built-in endpoints. Some providers use `models.dev` as a catalog host. No risk of it being missing.
- The existing `OpenCodeEndpoints(providerHost string, includeOpenCodeAI bool)` signature is unchanged. The model provider endpoint is appended by the caller when it differs from the configured host.
- Wildcard-pattern providers (Bedrock, Azure, Vertex) cannot be supported via the known map because the exact host depends on user configuration. The `--egress-allow` escape hatch handles these.
- The exact provider ID prefixes used in `--model` come from models.dev. Verify against the live catalog or the OpenCode source (`provider.ts`) during implementation. The IDs listed here are best-effort from models.dev documentation.
