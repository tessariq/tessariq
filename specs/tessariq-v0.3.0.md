# Tessariq v0.3.0 Specification

**Status:** Draft  
**Scope:** Third release  
**Theme:** Make safe custom runtime creation a first-class workflow

## Release intent

Tessariq v0.3.0 is intended to verify:

- the official minimal reference runtime image is a useful base for real local workflows
- Tessariq can help users create safe custom runtime images without taking ownership of upstream third-party agent version tracking
- runtime provenance and validation can stay clear and supportable as custom images become common

## Inheritance from v0.2.0

v0.3.0 inherits all v0.2.0 behavior unless this document changes it explicitly. In particular, these invariants still hold:

- `agent` remains the user-facing tool-selection concept
- the runtime image remains distinct from the selected agent
- Tessariq does not track or pin upstream third-party agent versions as a product responsibility
- supported-agent auth reuse remains read-only and MUST NOT expose host `HOME`
- `--mount-agent-config` remains an opt-in read-only config-dir mount for supported agents
- `runtime.json` remains the source of truth for runtime-image metadata

## Scope

v0.3.0 adds these normative capabilities:

- a Tessariq command to build a safe custom runtime image from an embedded Tessariq Dockerfile baseline
- stronger runtime provenance in `runtime.json`
- explicit runtime validation for custom image flows

Still out of scope:

- tracking upstream third-party agent versions automatically
- mirroring upstream third-party agent releases in a Tessariq-managed catalog
- devcontainer-derived runtime support
- arbitrary host-home passthrough

## Product intent

Tessariq v0.3.0 makes custom runtime images a first-class workflow without turning Tessariq into a package manager for third-party coding agents.

The product contract is:

- Tessariq provides a safe baseline runtime recipe
- users remain responsible for choosing and installing third-party agent binaries into derived images
- Tessariq records provenance and validates compatibility, but it does not manage upstream agent release cadence for the user

## CLI additions

### `tessariq runtime bake`

Intent:

- create a safe custom runtime image from an embedded Tessariq Dockerfile baseline

High-level contract:

- Tessariq MUST provide an embedded Dockerfile baseline that inherits the official Tessariq runtime safety posture
- the command MUST produce a local Docker image tag chosen by the user
- the command MUST allow user-controlled extension of the embedded baseline so the user can install desired third-party agent binaries or repo-specific tooling
- the command MUST NOT silently fetch or track third-party agent versions on the user's behalf

Detailed flag shape is intentionally left for task-level refinement.

## Runtime provenance

`runtime.json` MUST grow to support custom runtime provenance.

Minimum additional fields when a runtime is baked through Tessariq:

```json
{
  "schema_version": 1,
  "image": "example/custom-runtime:tag",
  "image_source": "baked",
  "bake_source": "embedded-dockerfile",
  "base_runtime": "ghcr.io/tessariq/reference-runtime:v0.1.0"
}
```

Rules:

- provenance fields MUST identify whether the runtime image came from the official reference image, a CLI override, or a Tessariq bake workflow
- provenance fields MUST NOT record secrets
- implementations MAY add image digests and additional provenance fields without changing `schema_version` if they preserve the meaning of existing fields

## Runtime validation

When a custom runtime image is used:

- Tessariq MUST validate that the selected agent binary exists before agent start
- Tessariq MUST preserve the existing read-only auth and config mount contracts
- validation failures MUST stay actionable and tell the user what is missing from the runtime image

## Acceptance scenarios

- `tessariq runtime bake` produces a new local runtime image from the embedded Tessariq Dockerfile baseline
- a baked runtime image can be used with `tessariq run --image ...`
- `runtime.json` records baked-runtime provenance clearly
- runtime validation catches a missing selected agent binary before agent start
- baked images preserve the same supported-agent auth reuse contract as the reference runtime flow

## Failure UX

| Condition | Required behavior | Required user guidance |
| --- | --- | --- |
| runtime bake cannot build the image | fail without producing a partial success record | identify the failing build step and tell the user how to inspect or rerun the build |
| baked runtime image is missing the selected agent binary | fail before agent start | identify the missing binary and tell the user to rebuild or extend the runtime image |
| baked runtime image conflicts with the read-only auth/config mount contract | fail before agent start | explain the incompatible runtime expectation and tell the user the baked image must support the documented Tessariq runtime contract |

## Success metrics

- at least 80% of users attempting custom runtime creation can produce a runnable image without editing Tessariq source code
- fewer than 15% of custom runtime failures are caused by unclear provenance or runtime validation messages

## Implementation Notes (Informative)

This section is informative. It describes the likely implementation shape for v0.3.0, and the normative sections above take precedence if there is any conflict.

### Embedded Dockerfile baseline

The embedded Dockerfile is expected to:

- start from the Tessariq reference runtime image or an equivalent safe baseline
- preserve the non-root and read-only-auth assumptions established earlier
- make it easy for users to add third-party agent binaries and extra repo tooling in explicit layers

### Deferred devcontainer support

Devcontainer-derived runtimes remain deferred beyond v0.3.0.

The expected later direction is:

- use devcontainer configuration as build input for a safe derived runtime
- preserve Tessariq safety policy rather than blindly running arbitrary devcontainers as-is
