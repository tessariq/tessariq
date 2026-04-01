# Adversarial Test Results — Spec v0.1.0

Automated adversarial testing against done tasks.

| Key | Value |
|-----|-------|
| Started | 2026-03-31 |
| Iteration | 7 |
| Last bug found | iteration 7 |
| Clean iterations | 0 / 6 |
| Status | In progress |

---

## Iteration 1

### BUG-001: OpenCode `--interactive` rejected at CLI instead of recorded as not-applied

**Severity:** HIGH

**Spec says** (line 267):
> if an option such as `--model` or `--interactive` cannot be applied exactly, the selected agent MUST record that it was requested but not applied

**Code does:** The CLI hard-rejects the combination at `cmd/tessariq/run.go:71-73`:
```go
if cfg.Interactive && cfg.Agent == "opencode" {
    return fmt.Errorf("--interactive is not supported by opencode; use --agent claude-code for interactive mode")
}
```
This error fires **before** the adapter is constructed, so `agent.json` never records the request.

**Impact:** The OpenCode adapter (`internal/adapter/opencode/opencode.go:73,85`) already correctly records `interactive: true` in `Requested()` and `interactive: false` in `Applied()`. The CLI gate prevents the adapter from ever running. Removing the CLI gate (lines 71-73) would make the code spec-compliant with no other changes needed.

**File:** `cmd/tessariq/run.go:71-73`

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| CLI flags vs spec | All 14 `tessariq run` flags, defaults, validation | CLEAN |
| Evidence schema | manifest/status/agent/runtime/workspace field-by-field | CLEAN |
| Evidence permissions | 0o600 files, 0o700 dirs | CLEAN |
| Container security | cap-drop, no-new-privileges, RO mounts, user | CLEAN |
| Error handling | dirty-repo, prereqs, failure evidence, status transitions | CLEAN |
| Unit tests | `go test ./...` | CLEAN (all packages pass) |
| Static analysis | `go vet ./...` | CLEAN |
| Formatting | `gofmt -l .` | CLEAN |

### Out-of-scope findings (TODO tasks, not bugs in done tasks)

- Repair container uses `alpine:latest` instead of digest-pinned image — tracked as TASK-031 (TODO)

---

## Iteration 2

### BUG-002: `--egress-allow` does not bypass OpenCode provider resolution despite being the recommended workaround

**Severity:** HIGH

**Spec says** (line 341):
> when OpenCode is selected and Tessariq cannot determine the provider host required for `--egress auto` from the available config and auth state, Tessariq MUST fail before container start and tell the user to configure the provider explicitly or use `--egress-allow`

The spec recommends `--egress-allow` as the escape hatch when provider auto-detection fails.

**Code does:** At `cmd/tessariq/run.go:231-237`, provider resolution runs unconditionally whenever `resolvedEgress == "proxy"`, regardless of whether the user passed `--egress-allow`:
```go
if resolvedEgress == "proxy" {
    // ...
    provInfo, provErr := opencode.ResolveProviderFromPaths(authPath, configDir, os.ReadFile)
    if provErr != nil {
        return nil, provErr
    }
    agentEndpoints = adapter.OpenCodeEndpoints(provInfo.Host, provInfo.IsOpenCodeHosted)
}
```

The error message at `internal/adapter/opencode/provider.go:22-23` says:
> "configure the provider explicitly so Tessariq can derive the required host, or pass --egress-allow manually"

But passing `--egress-allow api.openai.com:443` still fails because provider resolution fires before the allowlist merge at line 248. The suggested workaround does not actually work.

**Fix:** Skip provider auto-resolution when the user has provided explicit `--egress-allow` entries (i.e., check `len(cfg.EgressAllow) > 0` before attempting resolution).

**File:** `cmd/tessariq/run.go:231-237`, `internal/adapter/opencode/provider.go:22-23`

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| Timeout/grace handling | SIGTERM/SIGKILL, timeout.flag, status field | CLEAN (TASK-030 out of scope) |
| Auth/config mounts | All paths, RO enforcement, HOME isolation, runtime.json | CLEAN |
| Interactive runtime mode | Activity timer, TTY flags, CLI args, warnings | CLEAN |
| Acceptance scenarios | Init, dirty-repo, prereqs, container naming, evidence | CLEAN |

### Out-of-scope findings (TODO tasks, not bugs in done tasks)

- Timeout signal escalation sends SIGKILL directly instead of SIGTERM first — tracked as TASK-030 (TODO)

---

## Iteration 3

### BUG-003: `tessariq init` creates `.tessariq/runs/` with 0o755 instead of 0o700

**Severity:** HIGH

**Spec says** (evidence contract, security hardening amendments 2026-03-31):
> Evidence directories MUST be created with permissions `0o700` (owner-only access). Evidence is intended for the invoking user only and MUST NOT be world-readable.

**Code does:** At `internal/initialize/initialize.go:16`:
```go
if err := os.MkdirAll(runsDir, 0o755); err != nil {
```
Creates `.tessariq/` and `.tessariq/runs/` with `0o755` (world-readable and world-searchable).

**Impact:** While per-run evidence directories are correctly created with `0o700` (via `internal/run/manifest.go:56`), the parent directories allow any system user to enumerate run IDs (ULIDs) by listing `.tessariq/runs/`. This is an information leak — run IDs are sensitive because they can be used to reference runs and appear in container names and tmux sessions.

**Fix:** Change `0o755` to `0o700` at `initialize.go:16`.

**File:** `internal/initialize/initialize.go:16`

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| Worktree provisioning | git worktree add --detach, cleanup, chmod, workspace.json | CLEAN |
| Container lifecycle | docker create+start, naming, cleanup, security flags | CLEAN |
| Tmux session handling | Session creation, naming, detached guidance | CLEAN |
| Init idempotency | .gitignore handling, directory creation, re-run safety | CLEAN (functional) |
| Manifest & ULID | All 12 fields, ULID generation, types, permissions | CLEAN |

### Out-of-scope findings (TODO tasks, not bugs in done tasks)

- `--attach` auto-attach after start not implemented — tracked as TASK-007 (TODO)
- `tessariq attach` command referenced in run output not yet implemented — TASK-007 (TODO)
- Tmux sessions not cleaned up after run — no task currently tracks this

---

## Iteration 4

### BUG-004: base_sha retrieved independently in manifest and workspace, allowing divergence

**Severity:** MEDIUM

**Spec says** (line 106):
> worktree workspace mode has "Strong" reproducibility guaranteed "from `base_sha` on a clean repo"

Evidence artifacts must be internally consistent — manifest.json.base_sha and workspace.json.base_sha should always agree.

**Code does:** `git.HeadSHA()` is called twice independently:
1. `cmd/tessariq/run.go:87` — captured and written to manifest.json via `BootstrapManifest()`
2. `internal/workspace/provision.go:24` — captured again and written to workspace.json

Between these calls (~10-50ms window), another process could commit to the repo. The `baseSHA` from `Provision()` is returned as a second value (`wsPath, _, err`) but the caller ignores it with `_`.

**Impact:** If HEAD advances between the two calls, manifest.base_sha and workspace.base_sha diverge. The worktree is checked out at a different SHA than the manifest records. This breaks the reproducibility guarantee.

**Fix:** Pass the already-captured `baseSHA` from `run.go:87` into `workspace.Provision()` instead of having it call `git.HeadSHA()` again.

**File:** `cmd/tessariq/run.go:87,113`, `internal/workspace/provision.go:24`

### BUG-005: Agent binary pre-start validation not implemented (TASK-024/025 acceptance criteria)

**Severity:** HIGH

**Spec says** (TASK-024 acceptance criteria, line 60):
> Tessariq validates that the `claude` binary is present in the resolved runtime image before agent start.

And (TASK-025 acceptance criteria, line 60):
> Tessariq validates that the `opencode` binary is present in the resolved runtime image before agent start.

Also (spec failure UX, line 531):
> the selected agent binary is missing from the resolved runtime image → fail before agent start → identify the missing binary, name the selected agent, and tell the user to use a compatible runtime image or `--image` override

**Code does:** The factory at `internal/adapter/factory.go:67` sets `Command: append([]string{binaryName}, args...)` and starts the container. If the binary is missing, the container fails with exit code 127. Integration tests (`claudecode_integration_test.go:22`, `opencode_integration_test.go:22`) verify exit code 127 detection — but this is reactive detection **after** container start, not proactive validation **before** agent start as the spec requires.

**Impact:** Users get a generic container failure instead of the spec-required actionable error message identifying the missing binary and suggesting `--image` override.

**File:** `internal/adapter/factory.go:67`, `cmd/tessariq/run.go:147`

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| Config validation | Timeout/grace bounds, agent/egress values, flag combos | CLEAN |
| Task title extraction | Empty files, multiple H1, formatting, fallback | CLEAN |
| Agent factory wiring | Dispatch, CLI args, evidence recording per agent | CLEAN |
| Cross-field evidence | container_name, timestamps, schema_version, agent field | CLEAN |

### Minor findings (defensible design choices, not spec violations)

- `--unsafe-egress --egress open` rejected as mutually exclusive despite spec calling it an "alias" (`config.go:63-65`) — MEDIUM, spec ambiguity
- Case sensitivity for agent/egress values not normalized — LOW
- Empty `--egress-allow` values not rejected — LOW

---

## Iteration 5

### BUG-006: No cleanup defer for worktree after Provision — worktree leaked on subsequent errors

**Severity:** CRITICAL

**Spec says** (worktree contract):
> Always `git worktree remove` during cleanup. Worktree cleanup must happen even on failure.

**Code does:** After `workspace.Provision()` at `cmd/tessariq/run.go:113`, there is **no** `defer workspace.Cleanup(...)` call. Multiple error paths can return after the worktree is created without ever cleaning it up:
- Line 118-121: Auth discovery failure
- Line 130-133: Config directory discovery failure
- Line 147-151: Agent process creation failure
- Line 153-155: Write agent info failure
- Line 157-159: Write runtime info failure
- Line 177-179: Runner execution failure

Each of these leaves a dangling worktree directory at `~/.tessariq/worktrees/<repo_id>/<run_id>` AND a stale git worktree entry in the repo's worktree list.

**Impact:** Over time, failed runs accumulate orphaned worktrees that consume disk space and pollute `git worktree list` output. The git worktree entries prevent creating new worktrees with conflicting paths.

**Fix:** Add `defer workspace.Cleanup(ctx, homeDir, root, runID, wsPath)` immediately after line 116.

**File:** `cmd/tessariq/run.go:113-179`

### BUG-007: Logs not capped and no truncation marker

**Severity:** HIGH

**Spec says** (line 379):
> Logs MUST be capped and MUST include a truncation marker if truncated.

**Code does:** `internal/runner/logs.go:16-29` creates `run.log` and `runner.log` as unbounded `os.File` handles. Container output is streamed directly to `run.log` via `docker logs` (`internal/container/process.go:155-178`) with no size enforcement. A runaway container can produce unbounded output.

**Impact:** No maximum log size. A misbehaving agent could fill the disk. No truncation marker means consumers cannot distinguish between a complete log and one that hit a limit.

**File:** `internal/runner/logs.go:16-29`, `internal/container/process.go:155-178`

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| Worktree parent dir permissions | ~/.tessariq/worktrees/ at 0o755 | CLEAN (spec allows for bind-mount) |
| ResolveAllowlist precedence | CLI > user_config > built_in, source recording | CLEAN |
| status.json atomicity | Temp file + rename, crash safety, field types | CLEAN |

### Minor findings

- `--egress-allow` with `--egress open` accepted but meaningless — LOW, spec doesn't explicitly forbid it

---

## Bug Summary

BUG-001 through BUG-007 were all fixed by TASK-030 through TASK-039 (done).

| Bug | Severity | File | One-liner | Status |
|-----|----------|------|-----------|--------|
| BUG-001 | HIGH | `run.go:71-73` | OpenCode `--interactive` hard-rejected instead of recorded | **Fixed** (TASK-033) |
| BUG-002 | HIGH | `run.go:231-237` | `--egress-allow` doesn't bypass provider resolution | **Fixed** (TASK-034) |
| BUG-003 | HIGH | `initialize.go:16` | Init creates evidence parent dirs with 0o755 not 0o700 | **Fixed** (TASK-035) |
| BUG-004 | MEDIUM | `run.go:87,113` | base_sha TOCTOU between manifest and workspace | **Fixed** (TASK-036) |
| BUG-005 | HIGH | `factory.go:67` | Agent binary not validated before container start | **Fixed** (TASK-037) |
| BUG-006 | CRITICAL | `run.go:113-179` | No cleanup defer — worktree leaked on post-provision errors | **Fixed** (TASK-038) |
| BUG-007 | HIGH | `logs.go:16-29` | Logs not capped, no truncation marker | **Fixed** (TASK-039) |
| BUG-008 | HIGH | `run.go:200` | Spurious "interactive mode without --attach" note on every default claude-code run | **Open** |
| BUG-009 | HIGH | `run.go:288` | OpenCode + proxy + user_config allowlist fails if auth.json missing | **Open** |
| BUG-010 | MEDIUM | `run.go:288` | OpenCode auth-missing produces raw I/O error instead of actionable message | **Open** |
| BUG-011 | MEDIUM | `run.go:324` | `appendIndexEntry` silently drops all errors; run can complete without an index entry | **Open** |
| BUG-012 | LOW | `specs/tessariq-v0.1.0.md` | Manifest example uses `"allowlist_source": "auto"` which contradicts normative text | **Open (spec doc)** |

---

## Iteration 7

Scope: post-TASK-039 codebase. All previous bugs confirmed fixed. Four new bugs found.

### BUG-008: Spurious "interactive mode without --attach" note fires on every default claude-code run

**Severity:** HIGH  
**File:** `cmd/tessariq/run.go:200`

```go
if agentProc.AgentInfo.Applied["interactive"] && !cfg.Attach {
    fmt.Fprintf(cmd.ErrOrStderr(), "note: interactive mode without --attach; use 'tmux attach -t %s' to provide approval input\n", sessionName)
}
```

`claudecode.buildApplied` always returns `{"interactive": true}` — semantically "the interactive setting was successfully applied" (correct for claude-code, which supports both modes). The condition misreads this as "interactive was requested as true", so it fires for **every** non-attach claude-code run, including the default `tessariq run` with no flags.

**Spec reference:**
> `--interactive` without `--attach` is valid but will cause the agent to block waiting for approval with no terminal attached.

The note is appropriate only when `--interactive` was actually requested by the user.

**Reproduction:**
```
tessariq run specs/task.md
# stderr: note: interactive mode without --attach; use 'tmux attach -t ...' to provide approval input
# Expected: no note (user did not request --interactive)
```

**Fix direction:** Change the condition to `cfg.Interactive && !cfg.Attach`.

---

### BUG-009: OpenCode + `--egress proxy` + user-config allowlist fails when auth.json is absent

**Severity:** HIGH  
**File:** `cmd/tessariq/run.go:288` (`resolveAllowlistCore`)

```go
case "opencode":
    if resolvedEgress == "proxy" && len(cfg.EgressAllow) == 0 && !cfg.EgressNoDefaults {
        // ← does NOT check whether user_config already provides an allowlist
        provInfo, provErr := opencode.ResolveProviderFromPaths(authPath, configDir, deps.readFile)
        if provErr != nil {
            return nil, provErr  // raw I/O error, not auth-missing guidance
        }
    }
```

Provider resolution is only skipped when CLI `--egress-allow` entries exist. It is **not** skipped when `user_config.EgressAllow` is non-empty — even though in that case the built-in allowlist (and therefore the provider host) is never used. If auth.json is absent, the run fails with a confusing file-not-found error before `authmount.Discover` can surface the user-friendly auth-missing message.

**Spec reference (allowlist precedence):**
> if one or more CLI `--egress-allow` values are provided, the resolved allowlist MUST contain exactly those CLI destinations  
> **otherwise, if user-level config defines a default allowlist, the resolved allowlist MUST contain exactly the configured destinations**

When user_config takes precedence, built-in provider resolution is irrelevant and must not gate the run on auth.json being present.

**Reproduction:**
```yaml
# ~/.config/tessariq/config.yaml
egress_allow:
  - api.openai.com:443
```
```
# no ~/.local/share/opencode/auth.json
tessariq run --agent opencode specs/task.md
# → "read auth file: open .../auth.json: no such file or directory"
# Expected: run proceeds using user_config allowlist
```

**Fix direction:** Add `&& (userCfg == nil || len(userCfg.EgressAllow) == 0)` to the provider-resolution guard at `run.go:288`. `userCfg` is already loaded before the switch.

---

### BUG-010: OpenCode auth-missing error surfaces as a raw I/O error instead of the actionable spec message

**Severity:** MEDIUM  
**File:** `cmd/tessariq/run.go:288–296` (`resolveAllowlistCore`)

When OpenCode uses `--egress auto` or `--egress proxy` with no CLI or user_config allowlist, `ResolveProviderFromPaths` reads auth.json to resolve the provider host. If auth.json does not exist the error returned is:

```
read auth file: open /home/user/.local/share/opencode/auth.json: no such file or directory
```

`resolveRunAllowlist` is called before `authmount.Discover`, so the user-friendly `AuthMissingError` path is never reached.

**Spec reference (Failure UX table):**
> required supported agent auth state is missing → fail before agent start → identify that supported auth files or directories for the selected agent were not found and tell the user to **authenticate that agent locally first**

**Fix direction:** Check auth file existence before calling `ResolveProviderFromPaths` and surface `AuthMissingError` when it is absent, or re-order the calls so `authmount.Discover` runs first.

---

### BUG-011: `appendIndexEntry` silently swallows all errors; a run can complete with no index entry

**Severity:** MEDIUM  
**File:** `cmd/tessariq/run.go:324–338`

```go
func appendIndexEntry(repoRoot, evidenceDir string) {
    manifest, err := run.ReadManifest(evidenceDir)
    if err != nil {
        return  // silent drop
    }
    status, err := runner.ReadStatus(evidenceDir)
    if err != nil {
        return  // silent drop
    }
    _ = run.AppendIndex(runsDir, entry)  // silent drop
}
```

If status.json was not written (early runner failure before `WriteStatus`) or the filesystem is full, the run completes without an index entry. The user sees no warning. Subsequent `tessariq attach last` or `tessariq promote last` will either resolve to the wrong run or fail with `ErrEmptyIndex`.

**Spec reference:**
> `index.jsonl` is append-only; each line represents one run and MUST not be rewritten in place for another run.

The MUST implies entries are expected to be durably written; silent failure violates this.

**Fix direction:** Print a warning to stderr when any step fails:
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "warning: failed to append run index entry: %s\n", err)
}
```

---

### BUG-012 (spec doc): Manifest example uses `"allowlist_source": "auto"`, contradicting normative text

**Severity:** LOW  
**File:** `specs/tessariq-v0.1.0.md` — minimum `manifest.json` shape

```json
"allowlist_source": "auto",
```

**Normative text in the same spec:**
> `allowlist_source` in `manifest.json` and `egress.compiled.yaml` MUST be one of `cli`, `user_config`, or `built_in`

The implementation correctly uses `built_in` as the fallback source. The spec example is wrong and may mislead readers or automated validators.

**Fix direction:** Change the example value from `"auto"` to `"built_in"` in the spec.
