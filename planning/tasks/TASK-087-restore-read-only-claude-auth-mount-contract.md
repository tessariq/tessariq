---
id: TASK-087-restore-read-only-claude-auth-mount-contract
title: Enforce a secure host auth and config mount contract across all supported agents
status: todo
priority: p0
depends_on:
    - TASK-023-supported-agent-auth-mounts
    - TASK-024-claude-code-agent-runtime-integration
    - TASK-025-opencode-agent-runtime-integration
    - TASK-026-mount-agent-config-flag-and-config-dir-mounts
    - TASK-027-container-lifecycle-and-mount-isolation
milestone: v0.1.0
spec_version: v0.1.0
spec_refs:
    - specs/tessariq-v0.1.0.md#product-intent
    - specs/tessariq-v0.1.0.md#agent-and-runtime-contract
    - specs/tessariq-v0.1.0.md#acceptance-scenarios
    - specs/tessariq-v0.1.0.md#failure-ux
updated_at: "2026-04-13T21:30:00Z"
areas:
    - auth
    - security
    - adapters
    - container
    - evidence
verification:
    unit:
        required: true
        commands:
            - go test ./...
        rationale: Auth-discovery, runtime-state preparation, and runtime-evidence behavior should first be pinned with deterministic unit coverage.
    integration:
        required: true
        commands:
            - go test -tags=integration ./...
        rationale: The bug is about actual mount flags and host/container interaction, not only struct values.
    e2e:
        required: true
        commands:
            - go test -tags=e2e ./...
        rationale: v0.1.0's auth-reuse contract is user-visible and security-sensitive.
    mutation:
        required: true
        commands:
            - gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70
        rationale: A shallow fix could update evidence text without removing the writable host persistence path.
    manual_test:
        required: true
        commands: []
        rationale: A built CLI check should prove the container cannot persist agent state mutations back onto live host auth or config paths for any supported agent.
---

## Summary

Establish one secure, reusable host-mount strategy for all supported agents so Tessariq never gives an agent container writable access to live host auth, state, or config paths.

The immediate reproduced bug is Claude Code's writable `~/.claude.json` mount (BUG-050), but the implementation must land as a shared disposable runtime-state layer that prevents the same class of regression for OpenCode and any future supported agent.

## Supersedes

- BUG-050 from `planning/BUGS.md`.

## Design

### Problem

Claude Code mutates `~/.claude.json` on every startup (numStartups counter, MCP state, feature flags). The current implementation mounts the host's live `~/.claude.json` read-write to accommodate this. That violates the v0.1.0 read-only mount contract and enables container-to-host persistence attacks via MCP server injection.

### Solution: disposable per-run runtime-state layer

All host auth, state, and config paths are mounted read-only. When an agent requires writable access to a state file at runtime, Tessariq satisfies those writes through a disposable per-run copy, never through a writable host bind.

Concrete flow:

1. **Classify adapter inputs.** Each adapter declares:
   - immutable host inputs (auth files, config dirs, state files)
   - writable runtime targets (container paths the agent may need to mutate)

2. **Mount host inputs read-only.** All host auth/config/state paths use read-only bind mounts. No exceptions.

3. **Materialize writable runtime view.** Before the agent starts:
   - For file-sized inputs that need writability: copy the host read-only input into a tmpfs-backed container path at the exact location the agent expects.
   - The existing `WritableDirs` tmpfs mechanism in `internal/container/process.go` already creates tmpfs mounts for intermediate directories. The disposable copies land on these tmpfs mounts.
   - For directory-sized inputs: use a disposable per-run scratch directory if needed (not tmpfs).

4. **Run the agent against the disposable copy.** The agent can write freely. Writes land in tmpfs or scratch, never on the host.

5. **Discard on teardown.** The tmpfs evaporates when the container stops. No automatic sync back to host.

### Adapter contract extension

Extend `authmount.MountSpec` (or a new struct) so adapters declare:

- `HostPath` — host source, always mounted read-only
- `ContainerPath` — where the agent expects the file
- `SeedIntoRuntime` — whether this input must be copied into the writable tmpfs before agent start

This replaces the current `ReadOnly bool` field with a higher-level policy:

| `SeedIntoRuntime` | Host mount | Agent-visible path | Writable? |
|---|---|---|---|
| `false` | RO bind | `ContainerPath` | No |
| `true` | RO bind at seed path | tmpfs-backed copy at `ContainerPath` | Yes (disposable) |

### Changes per adapter

**Claude Code:**

| File | Before | After |
|---|---|---|
| `~/.claude/.credentials.json` | RO bind | RO bind (unchanged) |
| `~/.claude.json` | **RW bind** (BUG) | RO bind + disposable tmpfs copy |

**OpenCode:**

| File | Before | After |
|---|---|---|
| `~/.local/share/opencode/auth.json` | RO bind | RO bind (unchanged) |

OpenCode has no writable host inputs today, but the shared mechanism is available if a future version needs it.

### `runtime.json` alignment

`auth_mount_mode` remains `"read-only"` for all agents because it records the host-side policy. The disposable tmpfs copy is an implementation detail, not a mount-mode change. Remove the misleading hard-coded constant and derive the value from the actual mount assembly.

## Acceptance Criteria

- No supported agent uses a writable bind mount from the container into a live host auth, state, or config path during normal v0.1.0 runs.
- `runtime.json.auth_mount_mode` truthfully reflects the actual host-side mount policy for every supported agent.
- Claude Code's `~/.claude.json` is no longer writable from the container; startup mutations land in a disposable tmpfs-backed copy that is discarded after the run.
- OpenCode auth and config mounts remain read-only and are unaffected.
- The fix preserves the existing v0.1.0 rule that Tessariq does not expose the host `HOME` directory and does not broaden auth/config mount scope.
- Automated coverage proves that in-container writes do not persist into the host's real auth/config/state paths for both `claude-code` and `opencode`.
- Shared mount-assembly and validation logic makes it hard for future agent adapters to reintroduce writable host auth/config binds without test failures.
- The disposable runtime-state layer is the single mechanism for satisfying agent write needs; no adapter gets a special unsafe exception.

## Spec changes

This task includes the following changes to `specs/tessariq-v0.1.0.md`, which have already been applied:

1. **Agent and runtime contract** (normative paragraph added after the "fail cleanly" rule): disposable per-run runtime-state layer MUST be used when agents need writable state; host paths MUST NOT be writable from inside the container.
2. **Implementation notes — Supported auth and config paths**: describes the seed-then-copy pattern and states it applies uniformly across all supported agents.
3. **`runtime.json` schema example**: `auth_mount_mode` comment now clarifies that it records the host-side policy and that disposable copies are an implementation detail.
4. **Specification changelog**: entry dated 2026-04-13 documenting all three changes.

When implementing, verify that these spec amendments are still present and consistent with the final code. If the implementation deviates, update the spec accordingly and add a supplementary changelog entry.

## Test Expectations

- Start with a failing unit or integration test that shows the current Claude `.claude.json` mount is writable from the container side.
- Add shared coverage for runtime evidence so `runtime.json.auth_mount_mode` cannot drift from the actual mount implementation for any supported agent.
- Add integration or e2e coverage that attempts in-container writes and verifies the host-side auth/config/state paths remain unchanged for both supported agents.
- Add unit coverage for the disposable runtime-state preparation step (copy host input to tmpfs-backed path).
- Run mutation testing because this is a security-sensitive contract where evidence-only fixes are insufficient.

## TDD Plan

1. RED: capture the current writable Claude state mount and the misleading `runtime.json` contract in tests.
2. GREEN: extract the disposable runtime-state preparation mechanism — shared logic that copies host RO inputs into tmpfs-backed writable paths.
3. GREEN: wire Claude Code's `~/.claude.json` through the disposable layer instead of the writable host bind.
4. GREEN: align `runtime.json.auth_mount_mode` derivation with actual mount assembly.
5. GREEN: add shared validation that no adapter can declare a writable host bind without test failures.
6. VERIFY: rerun auth/runtime integration, e2e, and manual checks for both supported agents.

## Notes

- Keep the task anchored to the current v0.1.0 spec. Do not solve this by weakening the spec or normalizing writable host auth binds.
- BUG-050 is the reproduced trigger for this task, but the implementation must leave behind a shared invariant for all supported adapters rather than a one-off Claude special case.
- The existing `WritableDirs` tmpfs mechanism in `internal/container/process.go` already handles the tmpfs side. This task extends it with a seed-copy step, not a replacement.
- Do not add any mechanism for syncing disposable copies back to the host. The disposable layer is strictly one-directional: host -> tmpfs -> discard.
