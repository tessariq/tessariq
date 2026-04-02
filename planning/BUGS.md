# Adversarial Test Results — Spec v0.1.0

Automated adversarial testing against done tasks.

| Key | Value |
|-----|-------|
| Started | 2026-03-31 |
| Iteration | 15 |
| Last bug found | iteration 15 |
| Clean iterations | 0 / 14 |
| Status | In progress |

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
| BUG-008 | HIGH | `run.go:200` | Spurious "interactive mode without --attach" note on every default claude-code run | **Fixed** |
| BUG-009 | HIGH | `run.go:288` | OpenCode + proxy + user_config allowlist fails if auth.json missing | **Fixed** |
| BUG-010 | MEDIUM | `run.go:288` | OpenCode auth-missing produces raw I/O error instead of actionable message | **Fixed** |
| BUG-011 | MEDIUM | `run.go:324` | `appendIndexEntry` silently drops all errors; run can complete without an index entry | **Fixed** |
| BUG-012 | LOW | `specs/tessariq-v0.1.0.md` | Manifest example uses `"allowlist_source": "auto"` which contradicts normative text | **Fixed (spec doc)** |
| BUG-013 | CRITICAL | `index.go`, `promote.go` | `promote` trusts forged `evidence_path` entries and can promote out-of-repo evidence | **Fixed** |
| BUG-014 | HIGH | `completeness.go`, `promote.go` | `promote` ignores missing `diffstat.txt` even though spec requires it when changes exist | **Fixed** |
| BUG-015 | HIGH | `promote.go` | `promote` allows manifest tampering to rewrite branch identity and commit trailers | **Fixed** (TASK-048) |
| BUG-016 | HIGH | `attach.go` | `attach` trusts forged `evidence_path` entries from outside the repo | **Fixed** |
| BUG-017 | HIGH | `attach.go` | `attach` does not verify evidence directory belongs to the same `run_id` | **Fixed** (TASK-052) |
| BUG-018 | HIGH | `preflight.go` | `attach` depends on `git` but does not preflight it as a required prerequisite | **Fixed** (TASK-050) |
| BUG-019 | HIGH | `run.go` | `run` loads user config even when explicit CLI egress settings should bypass it | **Fixed** (TASK-053) |
| BUG-020 | HIGH | `runref.go` | `last-N` resolves index lines, not runs; duplicate entries break previous-run selection | **Fixed** |
| BUG-021 | HIGH | `index.go`, `runref.go` | `ReadIndex` accepts partial JSON objects as valid runs | **Fixed** |
| BUG-022 | HIGH | `taskpath.go` | `run` accepts symlinked task path whose real target is outside the repository | **Fixed** |
| BUG-023 | CRITICAL | `run.go`, `factory.go`, `process.go` | `--egress none` does not disable networking; container gets full internet access | **Fixed** |
| BUG-024 | MEDIUM | `run.go:216` | `WriteDiffArtifacts` error silently discarded; run completes without diff evidence | **Fixed** |
| BUG-025 | HIGH | `allowlist.go:39`, `squidconf.go:47` | Newline/control char injection in allowlist host corrupts Squid config | **Open** (TASK-058) |
| BUG-026 | MEDIUM | `allowlist.go:35-43` | Leading-dot hosts enable Squid wildcard subdomain matching | **Open** (TASK-059) |
| BUG-027 | HIGH | `process.go:104`, `runner.go:176` | `docker stop --time=10` makes `--grace` flag dead code | **Open** (TASK-060) |
| BUG-028 | MEDIUM | `provision.go:53-69` | Worktree and git ref leak when Docker unavailable during cleanup | **Open** (TASK-061) |
| BUG-029 | MEDIUM | `squid.go:56-59` | Squid proxy container lacks security hardening (no cap-drop, no-new-privileges) | **Open** (TASK-062) |
| BUG-030 | MEDIUM | `squid.go:16` | Squid proxy image uses unpinned `:latest` tag | **Open** (TASK-063) |
| BUG-031 | HIGH | `squidconf.go:52` | Squid ACL cross-product allows unintended host:port combinations | **Open** (TASK-064) |
| BUG-032 | MEDIUM | `allowlist.go:22` | IPv6 address misparse in ParseDestination | **Open** (TASK-065) |
| BUG-033 | MEDIUM | `diff.go:27` | Binary file changes silently dropped during promote (missing `--binary`) | **Open** (TASK-066) |
| BUG-034 | MEDIUM | `squid.go:49-103`, `topology.go:71` | Squid container and network leak on partial StartSquid failure | **Open** (TASK-067) |
| BUG-035 | LOW | `manifest.go:80` | WriteManifest not atomic; partial write on crash corrupts evidence | **Open** (TASK-068) |
| BUG-036 | LOW | `config.go:72` | `--egress open` silently discards `--egress-allow` without warning | **Open** (TASK-069) |

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

---

## Iteration 8

Scope: adversarial review of `tessariq promote` against the v0.1.0 spec.

### BUG-013: `promote` trusts forged `evidence_path` entries and can promote out-of-repo evidence

**Severity:** CRITICAL  
**Files:** `internal/run/index.go:77-108`, `internal/promote/promote.go:42-49`, `internal/promote/promote.go:64-99`

**Spec references:**
- `specs/tessariq-v0.1.0.md:68`
- `specs/tessariq-v0.1.0.md:214-246`
- `specs/tessariq-v0.1.0.md:482-494`

**Why this is a bug:**
- The spec defines run evidence as repo-local under `<repo>/.tessariq/runs/<run_id>/`.
- `promote` resolves a run from `index.jsonl` and then trusts `evidence_path` verbatim.
- If `evidence_path` is absolute, it is used as-is. If it is relative, it is joined to the repo root without checking that it remains under `.tessariq/runs/`.
- A forged or corrupted index entry can therefore point at attacker-controlled evidence outside the repo and still produce a real branch and commit.

**Adversarial test:**
1. Create a clean repo with a valid base commit.
2. Append an `index.jsonl` entry whose `evidence_path` points to an absolute temp directory outside the repo.
3. In that temp directory, create fake `manifest.json`, terminal `status.json`, required stub evidence files, and a `diff.patch` that adds a file.
4. Run `tessariq promote <run-id>`.
5. Expected by spec: fail because the run evidence is not repo-local.
6. Actual from code: the command can create a branch and commit from the external patch.

---

### BUG-014: `promote` ignores missing `diffstat.txt` even though v0.1.0 requires it when changes exist

**Severity:** HIGH  
**Files:** `internal/runner/completeness.go:10-21`, `internal/promote/promote.go:47-49`, `internal/promote/promote.go:64-71`

**Spec references:**
- `specs/tessariq-v0.1.0.md:363-374`
- `specs/tessariq-v0.1.0.md:246`
- `specs/tessariq-v0.1.0.md:516`
- `specs/tessariq-v0.1.0.md:540`

**Why this is a bug:**
- The spec says both `diff.patch` and `diffstat.txt` are required when a run has changes.
- Evidence completeness checks only the fixed evidence file set and does not require `diffstat.txt`.
- `promote` separately checks only whether `diff.patch` exists and is non-empty.
- A changed run with missing `diffstat.txt` still promotes successfully, which violates the required-evidence contract.

**Adversarial test:**
1. Create evidence for a finished run with a valid non-empty `diff.patch`.
2. Delete `diffstat.txt`.
3. Run `tessariq promote <run-id>`.
4. Expected by spec: fail and identify the missing artifact.
5. Actual from code: `promote` can still succeed.

---

### BUG-015: `promote` allows manifest tampering to rewrite branch identity and commit trailers

**Severity:** HIGH  
**Files:** `internal/run/manifest.go:55-67`, `internal/promote/promote.go:51-61`, `internal/promote/promote.go:73-82`, `internal/promote/promote.go:157-167`

**Spec references:**
- `specs/tessariq-v0.1.0.md:65-78`
- `specs/tessariq-v0.1.0.md:219-234`

**Why this is a bug:**
- `promote <run-ref>` should promote that run and link the resulting commit back to the same run.
- The implementation reads `manifest.json` and trusts its values for `run_id`, `base_sha`, `task_path`, branch naming, and trailers.
- There is no consistency check between the resolved run reference, the index entry, the evidence directory name, and the manifest contents.
- If `manifest.json` is tampered with, `promote RUN_A` can create a branch and commit that claim to come from `RUN_B` or a different task path.

**Adversarial test:**
1. Create a valid indexed run `RUN_A`.
2. Edit `.tessariq/runs/RUN_A/manifest.json` so `run_id` becomes `RUN_B` and `task_path` becomes `evil.md`.
3. Keep a valid `base_sha` and `diff.patch`.
4. Run `tessariq promote RUN_A`.
5. Expected by spec: fail because the run identity in evidence is inconsistent.
6. Actual from code: the created branch and trailers follow the forged manifest values.

---

## Iteration 9

Scope: adversarial review of `tessariq attach` against the v0.1.0 spec.

### BUG-016: `attach` trusts forged `evidence_path` entries and can validate liveness from outside the repo

**Severity:** HIGH  
**Files:** `internal/attach/attach.go:46-66`

**Spec references:**
- `specs/tessariq-v0.1.0.md:65-68`
- `specs/tessariq-v0.1.0.md:93`
- `specs/tessariq-v0.1.0.md:207-212`
- `specs/tessariq-v0.1.0.md:482-494`

**Why this is a bug:**
- v0.1.0 defines run evidence as repo-local under `<repo>/.tessariq/runs/<run_id>/`.
- `attach` resolves a run from `index.jsonl` and then trusts `entry.EvidencePath` verbatim.
- Absolute paths are accepted as-is, and relative paths are only joined to `repoRoot`; there is no check that the final path remains under `.tessariq/runs/<run_id>`.
- A forged or corrupted index entry can therefore make `attach` read `status.json` from arbitrary host paths outside the repository.

**Adversarial test:**
1. Build the current binary: `go build -o /tmp/tessariq-attach-adversarial ./cmd/tessariq`.
2. Create a temp git repo and a forged `.tessariq/runs/index.jsonl` entry whose `evidence_path` is an absolute temp directory outside the repo.
3. Place a fake `status.json` with `"state": "running"` in that external directory.
4. Start a tmux session named `tessariq-<run_id>`.
5. Run `tessariq attach last` under `script` so tmux has a terminal.
6. Expected by spec: fail, because the referenced run evidence is not repo-local.
7. Actual from code: the attach client connects; `tmux list-clients` showed `tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV` for the forged run.

**Fix direction:** Derive the evidence directory from `run_id`, or at minimum reject absolute paths and verify the cleaned path stays under `<repo>/.tessariq/runs/<run_id>` before reading `status.json`.

---

### BUG-017: `attach` does not verify that the evidence directory belongs to the same `run_id`

**Severity:** HIGH  
**Files:** `internal/attach/attach.go:46-66`

**Spec references:**
- `specs/tessariq-v0.1.0.md:65-69`
- `specs/tessariq-v0.1.0.md:207-212`
- `specs/tessariq-v0.1.0.md:245`
- `specs/tessariq-v0.1.0.md:482-494`

**Why this is a bug:**
- A run is identified by one `run_id` and one evidence folder under `.tessariq/runs/<run_id>/`.
- `attach` uses `entry.RunID` to derive the tmux session name, but it uses `entry.EvidencePath` independently to decide whether the run is live.
- There is no consistency check that `entry.EvidencePath` actually points to the evidence directory for `entry.RunID`.
- A forged index entry can therefore authorize `attach RUN_A` using `RUN_B`'s running `status.json`, then attach to the tmux session for `RUN_A`.

**Adversarial test:**
1. Create a temp repo with an index entry for `RUN_A`.
2. Set that entry's `evidence_path` to `.tessariq/runs/RUN_B`.
3. Write `status.json` with `"state": "running"` only for `RUN_B`.
4. Start tmux session `tessariq-RUN_A`.
5. Run `tessariq attach RUN_A`.
6. Expected by spec: fail, because `RUN_A` does not have its own live evidence.
7. Actual from code: the attach client connects; `tmux list-clients` showed `tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAA` even though only `RUN_B` had running evidence.

**Fix direction:** Enforce `entry.EvidencePath == .tessariq/runs/<entry.RunID>` or validate the resolved evidence directory name against `entry.RunID` before status/session checks.

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| Attach unit coverage | `go test ./internal/attach ./cmd/tessariq -run 'Attach|ResolveLiveRun'` | CLEAN |
| Attach integration baseline | `go test -tags=integration ./cmd/tessariq -run '^TestIntegration_Attach'` | CLEAN |
| Spec-required finished-run failure | Existing integration coverage plus codepath review | CLEAN |
| Spec-required missing-tmux guidance | Existing unit/e2e coverage plus codepath review | CLEAN |

---

## Iteration 10

Scope: adversarial review of prerequisite enforcement and egress-config precedence in `tessariq attach` and `tessariq run` against the v0.1.0 spec.

### BUG-018: `attach` depends on `git` for repo resolution but does not preflight it as a required host prerequisite

**Severity:** HIGH  
**Files:** `internal/prereq/preflight.go:34-35`, `cmd/tessariq/attach.go:35-42`, `cmd/tessariq/init.go:35-38`

**Spec references:**
- `specs/tessariq-v0.1.0.md:54-60`
- `specs/tessariq-v0.1.0.md:207-212`
- `specs/tessariq-v0.1.0.md:530`

**Why this is a bug:**
- The spec defines `git` as a required host prerequisite for repository discovery.
- `tessariq attach` resolves the current repository via `git rev-parse --show-toplevel`.
- `RequirementsForCommand("attach")` only checks for `tmux`, so the command can pass prerequisite validation and then fail later with a raw missing-binary error from `repoRoot()`.
- That violates the requirement that prerequisite failures happen before dependent work and identify the missing dependency with actionable guidance.

**Adversarial test:**
1. Build the current binary: `go build -o /tmp/tessariq-adversarial ./cmd/tessariq`
2. Create a temp git repo.
3. Create a temp `PATH` directory containing only `tmux`.
4. Run `PATH=/tmp/tessariq-adv-attach-bin /tmp/tessariq-adversarial attach last`.
5. Expected by spec: fail as a prerequisite error that identifies missing `git` and tells the user to install or enable it.
6. Actual from code:

```text
resolve repository root: exec: "git": executable file not found in $PATH
```

**Fix direction:** Add `git` to `RequirementsForCommand("attach")`, or otherwise preflight repository-discovery dependencies before calling `repoRoot()`.

---

### BUG-019: `run` loads user config even when explicit CLI egress settings should bypass it entirely

**Severity:** HIGH  
**Files:** `cmd/tessariq/run.go:270-305`, `internal/run/userconfig.go:35-54`

**Spec references:**
- `specs/tessariq-v0.1.0.md:96-98`
- `specs/tessariq-v0.1.0.md:321-324`
- `specs/tessariq-v0.1.0.md:352-356`

**Why this is a bug:**
- v0.1.0 limits user-level config to default allowlist selection for proxy-based egress flows.
- CLI flags are the per-run source of truth and must override user defaults.
- `resolveAllowlistCore()` unconditionally loads and parses `config.yaml` before checking whether the run already has an explicit CLI egress mode or CLI `--egress-allow` entries that make user config irrelevant.
- A malformed or unreadable user config therefore blocks runs that should not need that config at all.

**Adversarial tests:**
1. Create a clean temp repo with a committed `task.md`.
2. Create malformed user config at `/tmp/tessariq-adv-xdg-open/tessariq/config.yaml`.
3. Run `XDG_CONFIG_HOME=/tmp/tessariq-adv-xdg-open /tmp/tessariq-adversarial run --egress open task.md`.
4. Expected by spec: ignore user config for explicit `open` egress and continue past config loading.
5. Actual from code:

```text
malformed config file /tmp/tessariq-adv-xdg-open/tessariq/config.yaml: yaml: line 1: did not find expected ',' or ']'; check YAML syntax
```

6. Run `XDG_CONFIG_HOME=/tmp/tessariq-adv-xdg-open /tmp/tessariq-adversarial run --egress proxy --egress-allow example.com:443 task.md`.
7. Expected by spec: CLI `--egress-allow` overrides user defaults, so malformed user config should not block allowlist resolution.
8. Actual from code: the same config-parse failure occurs before CLI precedence is applied.

**Fix direction:** Skip loading user config when the resolved mode does not consult defaults (`open`, `none`), and also skip it when CLI inputs already fully determine the allowlist (`--egress-allow`, or `--egress-no-defaults` with non-proxy egress).

---

## Iteration 11

Scope: adversarial review of the run-index contract and `run-ref` resolution semantics against the v0.1.0 spec.

### BUG-020: `last-N` resolves index lines, not runs, so duplicate entries for the latest run break previous-run selection

**Severity:** HIGH  
**Files:** `cmd/tessariq/run.go:199`, `cmd/tessariq/run.go:218`, `internal/run/runref.go:42-56`

**Spec references:**
- `specs/tessariq-v0.1.0.md:242-255`
- `specs/tessariq-v0.1.0.md:393`
- `specs/tessariq-v0.1.0.md:482-494`

**Why this is a bug:**
- The spec defines `last` and `last-N` as run references against the repository's run index.
- The same spec also says `index.jsonl` is append-only and that each line represents one run.
- The implementation appends one line when a run enters `running` and appends another line for the same `run_id` when the run finishes.
- `ResolveRunRef()` counts raw lines, not unique runs, so `last-1` can resolve to an earlier lifecycle entry for the latest run instead of the previous run.

**Adversarial test:**
1. Build the current binary: `go build -o /tmp/tessariq-adversarial ./cmd/tessariq`
2. Create a temp repo with an initial commit.
3. Create finished evidence for `RUN_A` and `RUN_B`.
4. Write `.tessariq/runs/index.jsonl` with these three lines in order: `RUN_A success`, `RUN_B running`, `RUN_B success`.
5. Run `/tmp/tessariq-adversarial promote last-1`.
6. Expected by spec: resolve the previous run, `RUN_A`.
7. Actual from code:

```text
branch: tessariq/01BRZ3NDEKTSV4RRFFQ69G5FAV
commit: b17cf9478980ab2773a459c3b6d99a192608b913
```

The produced commit was for `Task B` and added `from-run-b.txt`, proving that `last-1` selected the latest run again instead of the previous run.

**Fix direction:** Make `run-ref` resolution operate on unique `run_id` values rather than raw index lines, or stop emitting duplicate index entries for the same run.

---

### BUG-021: `ReadIndex` accepts partial JSON objects as valid runs, so `attach last` can target repo-root pseudo-evidence

**Severity:** HIGH  
**Files:** `internal/run/index.go:98-102`, `internal/run/runref.go:42-56`, `internal/attach/attach.go:46-53`

**Spec references:**
- `specs/tessariq-v0.1.0.md:254`
- `specs/tessariq-v0.1.0.md:391`
- `specs/tessariq-v0.1.0.md:482-494`
- `specs/tessariq-v0.1.0.md:538`

**Why this is a bug:**
- The spec requires the minimum `index.jsonl` shape fields to be present.
- `ReadIndex()` treats any syntactically valid JSON object as an `IndexEntry`, even when required fields such as `evidence_path`, `state`, and `created_at` are missing.
- `ResolveRunRef("last")` can therefore return a partial zero-value entry as if it were a real run.
- `attach` then joins the empty `evidence_path` to `repoRoot`, causing it to probe `/repo/status.json` as though the repository root itself were the run evidence directory.

**Adversarial test:**
1. Build the current binary: `go build -o /tmp/tessariq-adversarial ./cmd/tessariq`
2. Create a temp git repo.
3. Write `.tessariq/runs/index.jsonl` containing only:

```json
{"run_id":"01CRZ3NDEKTSV4RRFFQ69G5FAV"}
```

4. Run `/tmp/tessariq-adversarial attach last`.
5. Expected by spec: reject the corrupted index entry and fail as though no valid run was found.
6. Actual from code:

```text
run is not live: run 01CRZ3NDEKTSV4RRFFQ69G5FAV is not live; evidence path: /tmp/tessariq-adv-index-partial: read status: open /tmp/tessariq-adv-index-partial/status.json: no such file or directory
```

That output proves the partial line was treated as a real run and that `attach` read from the repository root instead of rejecting the invalid index entry.

**Fix direction:** Validate required `IndexEntry` fields before accepting an index line, and reject or ignore semantically incomplete entries during `ReadIndex()` / `ResolveRunRef()`.

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| Explicit `run_id` resolution | Codepath review plus existing `internal/run/runref_test.go` coverage | CLEAN |
| `last` / `last-N` basic indexing | Existing `internal/run/runref_test.go` happy-path coverage | CLEAN |
| Missing index file handling | Existing `ErrEmptyIndex` coverage plus command behavior review | CLEAN |
| Promote branch creation from valid finished evidence | Adversarial repo setup succeeded aside from the `last-1` selection bug | CLEAN |

---

## Iteration 12

Scope: adversarial review of `tessariq run <task-path>` path validation against the v0.1.0 repository-boundary contract.

### BUG-022: `run` accepts a symlinked task path whose real target is outside the repository

**Severity:** HIGH  
**Files:** `internal/run/taskpath.go:15-28`, `cmd/tessariq/run.go:66-84`, `internal/run/taskcopy.go:9-20`

**Spec references:**
- `specs/tessariq-v0.1.0.md:86-89`
- `specs/tessariq-v0.1.0.md:244`
- `specs/tessariq-v0.1.0.md:528`

**Why this is a bug:**
- The spec says `tessariq run <task-path>` accepts a Markdown file inside the current repository.
- `ValidateTaskPathLogic()` only validates the lexical path string after `filepath.Join`, not the real filesystem target.
- `ValidateTaskPath()` then uses `os.Stat`, which follows symlinks, so a repo-local symlink to an external Markdown file is treated as a valid in-repo regular file.
- `run` then reads that external file and copies its content into evidence as `task.md`, violating the repo-boundary contract.

**Adversarial test:**
1. Build the current binary: `go build -o /tmp/tessariq-adversarial ./cmd/tessariq`
2. Create `/tmp/tessariq-adv-external-task/outside.md` with `# External Task`.
3. Create a temp git repo whose tracked `specs/task.md` is a symlink to that external file.
4. Run `/tmp/tessariq-adversarial run --egress open specs/task.md`.
5. Expected by spec: fail before container start and say the task path is outside the repository.
6. Actual from code: task validation succeeds and the command proceeds until a later, unrelated runtime-image check:

```text
agent claude-code: binary "claude" not found in runtime image ghcr.io/tessariq/claude-code:latest; use a compatible runtime image or specify --image to override
```

7. The created evidence proves the external file was accepted:

`manifest.json`
```json
"task_path": "specs/task.md",
"task_title": "External Task"
```

`task.md`
```markdown
# External Task
```

**Fix direction:** Resolve the task path with `filepath.EvalSymlinks` or equivalent before boundary checks, and reject any task whose real target escapes the repository root.

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| Absolute task path rejection | Existing `ValidateTaskPathLogic_OutsideRepo` coverage plus codepath review | CLEAN |
| `..` relative-escape rejection | Existing `ValidateTaskPathLogic_RelativeEscape` coverage plus codepath review | CLEAN |
| Non-Markdown task rejection | Existing `ValidateTaskPathLogic_NotMarkdown` coverage plus codepath review | CLEAN |
| Ordinary in-repo Markdown task acceptance | Existing `ValidateTaskPath_ValidFile` coverage | CLEAN |

---

## Iteration 13

Scope: broad adversarial review of version, init, evidence contracts, container security, egress enforcement, hooks, dirty-repo checks, and promote edge cases. Verified status of all previously open bugs (BUG-008 through BUG-022). Two new bugs found.

### Previously open bugs — status update

BUG-008 through BUG-012 are now **fixed**:
- **BUG-008**: `printInteractiveNote` now uses `cfg.Interactive` (the user's flag) instead of `agentProc.AgentInfo.Applied["interactive"]`. See `run.go:366`.
- **BUG-009**: Provider resolution guard at `run.go:288-289` now includes `(userCfg == nil || len(userCfg.EgressAllow) == 0)`.
- **BUG-010**: Auth-missing error at `run.go:294-296` now wraps `os.ErrNotExist` as `&authmount.AuthMissingError{Agent: "opencode"}`.
- **BUG-011**: `appendIndexEntry` at `run.go:344-361` now emits `fmt.Fprintf(w, "warning: index entry skipped: %s\n", err)` instead of silent drops.
- **BUG-012**: Spec manifest example at `specs/tessariq-v0.1.0.md:417` now uses `"allowlist_source": "built_in"`.

BUG-013 through BUG-022 are all **still open** — confirmed by code review.

### BUG-023: `--egress none` does not disable networking; container gets full internet access

**Severity:** CRITICAL
**Files:** `cmd/tessariq/run.go:133`, `internal/adapter/factory.go:70-81`, `internal/container/process.go:194-196`

**Spec says** (line 313, plus spec changelog line 803):
> Modes: `none`, `proxy`, `open`, `auto`

The `--egress-no-defaults` rename rationale describes `--egress none` as the mode "which disables all network access." The spec lists `none` as a distinct mode from `open`, with `open` requiring explicit opt-in. `none` is the most restrictive mode.

**Code does:** Only `proxy` mode configures container networking. Both `none` and `open` fall through with `proxyEnv == nil`, leaving `networkName` as `""`. In `process.go:194-196`, when `NetworkName` is empty, no `--net` flag is emitted to `docker create`, so the container lands on the default Docker bridge network with **full unrestricted internet access**.

```go
// run.go:133 — only proxy mode sets up networking
if resolvedEgress == "proxy" { ... }

// factory.go:70 — networkName stays empty for none/open
var networkName string
if proxyEnv != nil {
    networkName = proxyEnv.NetworkName
}

// process.go:194 — empty NetworkName means default bridge
if p.cfg.NetworkName != "" {
    args = append(args, "--net", p.cfg.NetworkName)
}
```

**Impact:** Users relying on `--egress none` for air-gapped, fully isolated runs unknowingly give the agent container full internet access. This is a security issue because the user chose the most restrictive mode but gets the most permissive network posture (equivalent to `--egress open`).

**Fix direction:** When `resolvedEgress == "none"`, set `NetworkName` to `"none"` in the container config. Docker's built-in `none` network provides loopback only — no external connectivity.

---

### BUG-024: `WriteDiffArtifacts` error silently discarded; run can complete without required diff evidence

**Severity:** MEDIUM
**File:** `cmd/tessariq/run.go:216`

**Spec says** (line 373-374):
> `diff.patch` when there are changes
> `diffstat.txt` when there are changes

These are **required** artifacts per the evidence contract.

**Code does:**

```go
_ = runner.WriteDiffArtifacts(cmd.Context(), evidenceDir, wsPath, baseSHA)
```

The error return is silently discarded with `_`. If `git diff` fails (e.g., corrupted worktree, git crash, disk full), a run with actual code changes will complete with neither `diff.patch` nor `diffstat.txt`. The user sees no warning.

Combined with BUG-014 (completeness check doesn't validate conditional artifacts), and the separate `appendIndexEntry` path, a run can complete, appear in the index, and even be promotable despite missing required diff evidence.

**Fix direction:** At minimum, emit a warning to stderr when `WriteDiffArtifacts` returns an error:
```go
if err := runner.WriteDiffArtifacts(cmd.Context(), evidenceDir, wsPath, baseSHA); err != nil {
    fmt.Fprintf(cmd.ErrOrStderr(), "warning: diff artifacts not written: %s\n", err)
}
```

---

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| `tessariq version` | Both invocation forms, output format, no-repo context, no prerequisites | CLEAN |
| `tessariq --version` | Byte-for-byte identical to `tessariq version` | CLEAN |
| `tessariq init` | Idempotency, .gitignore handling, permissions 0o700, git-unavailable failure | CLEAN |
| Init edge cases | CRLF .gitignore, subdirectory invocation, read-only .gitignore, no commits | CLEAN |
| Evidence JSON schemas | All 5 artifacts: schema_version=1, required fields present | CLEAN |
| Evidence file permissions | All files 0o600, all dirs 0o700 | CLEAN |
| Log capping (BUG-007 fix) | CappedWriter at 50 MiB with truncation marker | CLEAN |
| Container security | cap-drop=ALL, no-new-privileges, non-root user, repair image pinned | CLEAN |
| Evidence mount read-only | Confirmed via AssembleMounts | CLEAN |
| Auth/config mounts read-only | All auth and config mounts set ReadOnly: true | CLEAN |
| HOST HOME isolation | Never mounted; container HOME is /home/tessariq | CLEAN |
| Hook execution | Runs on host via sh -c, output captured in runner.log | CLEAN |
| Pre-hook failure | Aborts run, writes StateFailed | CLEAN |
| Verify-hook failure | Records StateFailed, runs all verify commands | CLEAN |
| `--egress open` contract | Explicit opt-in, recorded in manifest, default bridge networking | CLEAN |
| `--unsafe-egress` alias | Maps to "open", validated against --egress conflicts | CLEAN |
| Built-in allowlist profile | npm, PyPI, RubyGems, crates.io, Go proxy, Maven, Wikipedia — all on 443 | CLEAN |
| Agent endpoints | Claude Code: 3 endpoints correct; OpenCode: provider-aware, conditional opencode.ai | CLEAN |
| Allowlist precedence | CLI > user_config > built_in, --egress-no-defaults | CLEAN |
| egress.compiled.yaml | schema_version=1, allowlist_source, destinations with host+port | CLEAN |
| Dirty-repo check | Staged, unstaged, untracked, .gitignored, submodules, before container start | CLEAN |
| Dirty-repo error message | Matches spec: "commit, stash, or clean the repository first" | CLEAN |
| Promote edge cases | Branch-exists, unfinished-run, --no-trailers, --branch, git add -A, zero-diff | CLEAN |
| Promote terminal state check | All 5 terminal states accepted, running rejected | CLEAN |

---

## Iteration 14

Scope: deep adversarial review across three axes — (1) path traversal and input validation, (2) process lifecycle and concurrency, (3) security isolation and container hardening. Verified status of all previously open bugs (BUG-013 through BUG-024).

### Previously open bugs — status update

BUG-013, BUG-014, BUG-016, BUG-020, BUG-021, BUG-022, BUG-023, BUG-024 are now **fixed**:
- **BUG-013**: `promote.go` now calls `run.ValidateEvidencePath(repoRoot, entry.EvidencePath)` before processing.
- **BUG-014**: `promote.go` now checks `diffstat.txt` existence and non-emptiness.
- **BUG-016**: `attach.go` now calls `run.ValidateEvidencePath(repoRoot, entry.EvidencePath)`.
- **BUG-020**: `runref.go` now calls `uniqueRuns(entries)` to deduplicate by run_id before indexing.
- **BUG-021**: `index.go` now requires `entry.isComplete()` with all 8 required fields.
- **BUG-022**: `taskpath.go` now calls `filepath.EvalSymlinks` on both task path and repo root.
- **BUG-023**: `factory.go` sets `NetworkName: "none"` for egress none; `process.go` passes `--net none`.
- **BUG-024**: `run.go` now calls `warnDiffArtifacts` which emits a warning to stderr.

BUG-015, BUG-017, BUG-018, BUG-019 are **still open**:
- **BUG-015**: Manifest still read without integrity verification; local user can tamper branch identity.
- **BUG-017**: Evidence path validated under `.tessariq/runs/` but directory name not cross-checked against `entry.RunID`.
- **BUG-018**: `attach` still does not preflight `git` as a required host prerequisite.
- **BUG-019**: `run` still loads user config even when `--egress none` makes the allowlist irrelevant.

### BUG-025: Newline/control character injection in egress allowlist host corrupts Squid config

**Severity:** HIGH
**File:** `internal/run/allowlist.go:39`, `internal/proxy/squidconf.go:47`

**What happens:** `ParseDestination` validates hosts only against space and tab (`strings.ContainsAny(host, " \t")`) but does not reject newlines (`\n`, `\r`) or other control characters. A host containing a newline passes validation and flows through to `GenerateSquidConf` where it is interpolated via `fmt.Fprintf(&b, "acl allowed_dest dstdomain %s\n", d.Host)`. This produces a multi-line output that can inject arbitrary Squid directives.

**Attack vector:** A user config YAML file with double-quoted strings interprets `\n` as a literal newline:
```yaml
egress_allow:
  - "evil.com\nhttp_access allow all\nacl x dstdomain"
```

This generates a squid.conf containing `http_access allow all` as a standalone directive *before* the `deny all` rule, effectively disabling the entire egress firewall.

**Impact:** Egress policy bypass via config file manipulation. While the primary attack surface is the user's own config, a compromised tool in the repo could write to `~/.config/tessariq/config.yaml`.

**Fix:** Replace the whitespace check with a hostname character allowlist (RFC 1123: `[a-zA-Z0-9.-]`) or at minimum reject all bytes < 0x20 and 0x7F.

### BUG-026: Leading-dot hosts in allowlist enable Squid wildcard subdomain matching

**Severity:** MEDIUM
**File:** `internal/run/allowlist.go:35-43`, `internal/proxy/squidconf.go:47`

**Spec says:** Allowlists are enforced at "host:port granularity."

**What happens:** `ParseDestination` does not reject leading-dot hostnames. A value like `.github.com` passes all validation checks and produces `acl allowed_dest dstdomain .github.com` in squid.conf. Squid's `dstdomain` interprets leading dots as wildcard subdomain matchers, matching **all** subdomains of `github.com` — violating the spec's host:port granularity promise.

**Reproduction:** `tessariq run --egress-allow .github.com tasks/sample.md` — allows `evil.github.com`, `anything.github.com`, etc.

**Fix:** Reject hosts starting with `.` in `ParseDestination`.

### BUG-027: `docker stop --time=10` in Signal() makes `--grace` flag dead code

**Severity:** HIGH
**File:** `internal/container/process.go:104`, `internal/runner/runner.go:176-192`

**What happens:** `Process.Signal(SIGTERM)` runs `docker stop --time=10`, which blocks synchronously until the container stops or 10 seconds elapse (then Docker sends SIGKILL). This call at `runner.go:176` is synchronous — by the time it returns, the container is already dead and `Wait()` has already sent its result on `waitCh`.

The runner's subsequent `select` at line 178-192 with `time.After(r.Config.Grace)` never fires because `waitCh` already has a value. Consequences:
1. **`--grace` flag is entirely ignored.** The actual grace period is always the hardcoded 10 seconds inside `docker stop`.
2. **The SIGKILL escalation path (lines 183-191) is unreachable dead code** — it can never execute with the Docker container backend.
3. `--grace 60s` gives 10s of grace; `--grace 1s` gives 10s of grace.

**Reproduction:** Run with `--grace 1s` and a task that hangs. After timeout, observe the container lives for 10 seconds (docker stop's hardcoded grace). The runner.log will never contain "grace period expired, sending SIGKILL".

**Fix:** Use `docker kill --signal=SIGTERM` (non-blocking) instead of `docker stop` for the SIGTERM step, and let the runner's own grace timer handle the SIGKILL escalation. Alternatively, pass `--time=<grace_seconds>` derived from `r.Config.Grace`.

### BUG-028: Worktree and git ref leak when Docker is unavailable during cleanup

**Severity:** MEDIUM
**File:** `internal/workspace/provision.go:53-69`

**What happens:** `Cleanup()` calls `repairWorkspaceOwnership()` first (line 58), which runs `docker run --rm alpine chown ...`. If Docker is unavailable (daemon crashed, image not cached on air-gapped host), this returns an error and Cleanup returns immediately (line 59) without reaching `git.RemoveWorktree` (line 62) or `os.RemoveAll` (line 68).

Both the git worktree reference and filesystem directory are leaked. Stale git worktree refs cause `git worktree add` to fail for the same path. The caller in `cmd/tessariq/run.go` only emits a warning.

**Reproduction:** Start a run successfully, then stop the Docker daemon before the run completes. When cleanup runs, `repairWorkspaceOwnership` fails. Run `git worktree list` to see the leaked ref.

**Fix:** Attempt `git.RemoveWorktree` and `os.RemoveAll` even when `repairWorkspaceOwnership` fails, perhaps with a host-side `chmod -R u+rwX` fallback.

### BUG-029: Squid proxy container lacks security hardening

**Severity:** MEDIUM
**File:** `internal/proxy/squid.go:56-59`

**What happens:** The Squid proxy container is created with no security flags: no `--cap-drop ALL`, no `--security-opt no-new-privileges`, no resource limits. This contrasts with the agent container (`process.go:194-195`) which correctly applies both `--cap-drop ALL` and `--security-opt no-new-privileges`.

The Squid container is a critical security boundary — it enforces the egress allowlist. It's on the internal network *and* the bridge network (added at `squid.go:87`). A vulnerability in Squid triggered by a malicious CONNECT request from the agent could be escalated more easily without capability dropping.

**Reproduction:** `docker inspect tessariq-squid-<id> | jq '.[0].HostConfig.CapDrop'` returns null instead of `["ALL"]`.

**Fix:** Add `--cap-drop`, `ALL`, `--security-opt`, `no-new-privileges` to the `docker create` call in `StartSquid`.

### BUG-030: Squid proxy image uses unpinned `:latest` tag

**Severity:** MEDIUM
**File:** `internal/proxy/squid.go:16`

**What happens:** `DefaultSquidImage` is `"ubuntu/squid:latest"` — a mutable, unpinned tag. Meanwhile the repair image in `workspace/provision.go:17` is correctly pinned by digest (`alpine@sha256:...`). A supply-chain attack on the `ubuntu/squid` image on Docker Hub would compromise every proxy-mode run.

**Fix:** Pin `DefaultSquidImage` by digest, matching the pattern already used for the repair image.

---

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| Task path validation | Symlinks, `../`, null bytes, absolute paths | CLEAN (fixed in BUG-022) |
| Evidence path validation | Absolute paths, `../`, symlinks, out-of-repo | CLEAN (fixed in BUG-013/016) |
| RunRef resolution | Malformed IDs, last-0, last-N with N > entries | CLEAN (fixed in BUG-020) |
| Index JSONL integrity | Incomplete entries, missing fields, flock locking | CLEAN (fixed in BUG-021) |
| Container cap-drop/no-new-privileges | Agent container security flags | CLEAN |
| Evidence mount read-only | `/evidence` mount flags | CLEAN |
| Auth/config mounts read-only | All auth mounts set ReadOnly: true | CLEAN |
| `--egress none` networking | Container gets `--net none` (loopback only) | CLEAN (fixed in BUG-023) |
| `WriteDiffArtifacts` error handling | Warning emitted to stderr | CLEAN (fixed in BUG-024) |
| Port validation | Port 0, 65536, negative, non-numeric | CLEAN |
| Docker container name collision | Deterministic `tessariq-<run_id>` | CLEAN |
| Log capping | CappedWriter correctness, truncation marker | CLEAN |
| Status artifact atomicity | JSON serialization, state transitions | CLEAN |

---

## Iteration 15

Scope: deep adversarial review across three new axes — (1) CLI argument parsing, duration handling, and flag conflicts, (2) git operations, promote logic, and diff artifact generation, (3) adapter/auth/proxy lifecycle and teardown paths.

### BUG-031: Squid ACL cross-product allows unintended host:port combinations

**Severity:** HIGH
**File:** `internal/proxy/squidconf.go:22-52`

**What happens:** `GenerateSquidConf` creates two separate Squid ACLs — `SSL_ports` for all unique ports and `allowed_dest` for all unique hosts — then combines them in a single rule: `http_access allow CONNECT SSL_ports allowed_dest`. Squid evaluates multiple values in same-named ACLs with OR logic and ANDs the different ACL names.

This means **any allowed host can be reached on any allowed port**, creating a cross-product. With allowlist `[api.openai.com:443, internal.example.com:8443]`, the generated config allows:
- `api.openai.com:443` (intended)
- `internal.example.com:8443` (intended)
- `api.openai.com:8443` (NOT intended)
- `internal.example.com:443` (NOT intended)

The spec says allowlists are enforced at "host:port granularity" but the generated config enforces at "(any host) x (any port)" granularity.

**Impact:** When the allowlist contains hosts on different ports, an agent can reach any allowed host on any allowed port. This widens the egress surface beyond what the user configured.

**Reproduction:** `tessariq run --egress proxy --egress-allow "api.openai.com:443" --egress-allow "internal.example.com:8443" task.md`. From inside the agent container, `CONNECT api.openai.com:8443` via the proxy succeeds even though only `:443` was allowed for that host.

**Fix:** Generate per-destination ACL pairs with unique names (e.g., `acl dest0 dstdomain api.openai.com` + `acl port0 port 443`, then `http_access allow CONNECT dest0 port0`), or use Squid's `note` or external ACL helpers for per-pair enforcement.

### BUG-032: IPv6 address misparse in ParseDestination

**Severity:** MEDIUM
**File:** `internal/run/allowlist.go:22`

**What happens:** `ParseDestination` uses `strings.LastIndex(s, ":")` to split host from port. IPv6 addresses contain colons as part of the address, so they are incorrectly parsed. For example:
- `"2001:db8::1"` → host=`"2001:db8:"`, port=`1` (should be host=`"2001:db8::1"`, port=`443`)
- `"[::1]:443"` → host=`"[::1]"` with brackets preserved, producing malformed Squid `dstdomain`

The function has no IPv6 awareness at all.

**Reproduction:** `tessariq run --egress proxy --egress-allow "2001:db8::1" task.md` silently misparses and produces a Squid config with `dstdomain 2001:db8:` which is not a valid address.

**Fix:** Use `net.SplitHostPort` for `host:port` forms and handle bare hostnames separately, or detect IPv6 by bracket presence.

### BUG-033: Binary file changes silently dropped during promote

**Severity:** MEDIUM
**File:** `internal/git/diff.go:27`, `internal/promote/promote.go:107`

**What happens:** The `Diff` function runs `git diff <baseSHA> -- .` without the `--binary` flag. By default, `git diff` outputs text-mode patches that represent binary files as "Binary files a/foo and b/foo differ" — no actual content. When promote later runs `git apply` on this patch, the apply succeeds but silently skips the binary hunks. The promoted branch is missing any binary file changes (images, compiled assets, etc.) the agent made during the run.

**Impact:** Silent data loss on promote. No error or warning is raised.

**Reproduction:** 1. Create a run where the agent adds/modifies a binary file (e.g., PNG). 2. The run completes and `diff.patch` is generated without binary content. 3. `tessariq promote last` — patch applies, but binary file is absent from the promoted branch.

**Fix:** Add `--binary` flag to the `git diff` command in `Diff()`.

### BUG-034: Squid container and network leak on partial StartSquid failure

**Severity:** MEDIUM
**File:** `internal/proxy/squid.go:49-103`, `internal/proxy/topology.go:71-73`

**What happens:** `StartSquid` performs a 5-step sequence (create, cp, network connect, start, readiness check). If steps 2-5 fail, the container created in step 1 is not cleaned up. The caller (`Topology.Setup` at `topology.go:71-73`) only calls `RemoveNetwork` on failure — it does NOT call `StopSquid`. Furthermore, `RemoveNetwork` itself fails silently because the orphaned container is still attached. The deferred `Teardown` in `cmd/tessariq/run.go` only runs after a successful Setup.

**Impact:** Orphaned Docker container and network persist until manually cleaned.

**Reproduction:** Use a broken squid.conf that causes Squid to crash before listening (readiness check times out). After the error, `docker ps -a | grep tessariq-squid` shows the leaked container.

**Fix:** Add cleanup logic to `StartSquid` or to the Setup error path to call `StopSquid` before `RemoveNetwork`.

### BUG-035: WriteManifest is not atomic — partial write corrupts evidence on crash

**Severity:** LOW
**File:** `internal/run/manifest.go:80`

**What happens:** `WriteManifest` calls `os.WriteFile` directly. If the process is killed mid-write (e.g., SIGKILL after grace timeout), the file contains partial JSON. This is inconsistent with `WriteStatus` which uses the tmp+rename atomic pattern. A corrupt `manifest.json` blocks promote permanently.

**Fix:** Use the same tmp+rename pattern as `WriteStatus`.

### BUG-036: `--egress open` silently discards `--egress-allow` without warning

**Severity:** LOW
**File:** `internal/run/config.go:72`

**What happens:** `Validate()` rejects `--egress-allow` with `--egress none` but does NOT reject or warn when combined with `--egress open`. In open mode, no proxy starts and the allowlist is silently ignored. The user believes they configured restrictions but gets unrestricted egress.

**Reproduction:** `tessariq run --egress open --egress-allow "example.com:443" task.md` — succeeds with no warning, but the allowlist is completely ignored.

**Fix:** Add a validation error or at minimum a warning when `--egress-allow` is provided with `--egress open`.

---

### Areas tested (clean)

| Area | Probe | Result |
|------|-------|--------|
| CLI flag parsing | All 14 `tessariq run` flags, defaults, types | CLEAN |
| Duration validation | Timeout > 0, Grace >= 0, Grace <= Timeout | CLEAN |
| Agent flag validation | Valid agent names, unknown agent rejection | CLEAN |
| Config struct validation | Required fields, field interactions | CLEAN |
| Git command injection | All git shell-outs use exec.Command args, not interpolation | CLEAN |
| Promote worktree lifecycle | Create, apply, commit, branch, remove | CLEAN |
| Index flock locking | Concurrent append correctness | CLEAN |
| Auth file discovery | Claude Code and OpenCode path resolution | CLEAN |
| Auth mount read-only enforcement | All MountSpec entries | CLEAN |
| Proxy teardown idempotency | StopSquid + RemoveNetwork on missing resources | CLEAN |
| Hook execution | sh -c with user-provided commands, output capture | CLEAN |
| Status write atomicity | tmp+rename pattern in WriteStatus | CLEAN |
