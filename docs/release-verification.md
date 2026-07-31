# Release verification — the human-only residue

The automated suites prove the mechanics of a release: `task test`,
`task test:integration`, `task test:e2e`, the nightly mutation gate,
`task licenses:check` and `task release:check` all run in CI. Every e2e test
drives a **fake agent binary** inside a container
(`internal/testutil/containers.StartRunEnv`), so no real agent — Claude Code or
OpenCode — has ever run in CI, and neither have real credentials, real model
calls, or real terminal rendering.

This document lists what a human must still run before and after tagging a
release, and why each case cannot be automated. Consult it per release instead of
re-deriving a manual plan from scratch or quietly skipping one.

Case IDs (`T1.2`, `T4.10`, …) are the stable identifiers from the v0.1.0 manual
test plan. Each case below is self-contained; the per-release readiness working
document (for v0.1.0: `docs/release-readiness-v0.1.0.md`, untracked) is historical
input, not a prerequisite.

## How to run and sign off

1. Cut the release commit and wait for CI to be green on it.
2. Build the CLI from that exact commit: `go build ./cmd/tessariq && ./tessariq version`.
3. Create a throwaway sandbox repo (one commit, clean tree) with a small task
   file that must produce a diff, and run the cases in section 1.
4. Tag, then run section 2 against the published artifacts.
5. Record, per case: result, tester, host OS and architecture, agent versions,
   and the release commit. Sign-off lives in the release pull request or the
   GitHub Release checklist for that version — not in this file, which stays
   version-independent.

A case that is skipped must be recorded as skipped with a reason. An unrecorded
case counts as not run.

## 1. Human-only cases before tagging

| Case | What a human verifies | Why it cannot be automated | Platforms |
|---|---|---|---|
| T1.2, T1.5, T2.1, T2.2 | Real Claude Code run end to end: `tessariq run <task>` returns immediately with all six required output lines; exactly one `tessariq-` container is live; the working tree stays clean; evidence completes; `tessariq promote last` yields exactly one branch and one commit with `Tessariq-*` trailers; `~/.claude.json` and `~/.claude/.credentials.json` hash identically before and after | Needs live host credentials and billed model calls against a third-party API. CI has no credentials, must not spend money, and must not depend on provider availability | linux/amd64; repeat on darwin/arm64 if darwin archives ship |
| T2.5, T2.6 | Real OpenCode run against a real provider; `--model` reaches the agent (visible in `run.log` or the agent banner) for both agents and is recorded in `agent.json` | Same as above — real provider credentials and billed calls | linux/amd64 |
| T1.3, T1.7 | `tessariq attach last` renders live agent output legibly with no garbled PTY, and `Ctrl-b d` detaches while the run continues; `tessariq run <task> --attach` attaches one-shot with no double-PTY garbling | Automation asserts that the client connects and bytes flow; whether a real agent's TUI is *readable* is a human judgement | linux/amd64; repeat on darwin/arm64 if darwin archives ship |
| T2.9 | `tessariq run <task> --interactive --attach`: the agent blocks for approval on tool use and approving in the attached terminal lets it proceed | Requires a human at the approval prompt. The BUG-008 half (no spurious interactive note on a plain run) is automated and needs no human pass | linux/amd64 |
| T2.3 | With `--mount-agent-config`, the agent actually consumes a config signal from the mounted `~/.claude/` — a distinguishing setting or MCP entry visible in the session | Automation proves the read-only mount and `CLAUDE_CONFIG_DIR` are present; only a real agent proves it reads them | linux/amd64 |
| T3.2 | OpenCode dual-host provider resolution under `--egress auto`: with `--model <provider>/<model>` where the provider differs from the configured one, **both** hosts appear in `egress.compiled.yaml` | The resolution logic is unit tested, but the input is a real on-disk provider configuration | linux/amd64 |
| T4.10 | Keychain-only Claude Code credentials with no file mirror: the run explains the v0.1.0 file-backed-credential limitation instead of failing opaquely | macOS-only credential storage | **darwin/arm64 only** |
| T5 | Bind-mount UID mapping on macOS: repeat T1.2, T1.5, T2.1, T2.2, T4.1 and T4.7 on darwin. Docker Desktop maps UIDs through VirtioFS; Linux preserves host UIDs untranslated, so the two platforms exercise materially different mount behavior | macOS cannot be containerized, and cannot legally be virtualized on non-Apple hardware | **darwin/arm64 only** |

Both darwin rows need Apple hardware. GitHub's macOS runners can host them, but
they cannot be reduced to a container, so they stay a scheduled human pass rather
than a per-pull-request gate.

## 2. Post-tag artifact verification

Human-only because they act on artifacts that only exist after publishing.

| Case | Check |
|---|---|
| T6.1 | `task release:dry` locally, then inspect `dist/` |
| T6.2 | Download each published archive, extract, run `./tessariq version` — it must print the tag version with a real commit and build date, never `unknown` |
| T6.3 | `sha256sum -c checksums.txt` against the published checksum file |
| T6.4 | From a machine with **no GHCR login**, `docker pull` each of `ghcr.io/tessariq/reference-runtime:<tag>`, `ghcr.io/tessariq/claude-code:<tag>`, `ghcr.io/tessariq/opencode:<tag>`. GHCR creates new packages private by default and there is no REST endpoint for package visibility, so this stays manual |
| T6.5 | Follow the README quickstart verbatim on a fresh machine. Any step requiring knowledge not in the README is a documentation bug |
| T6.6 | Confirm the published GitHub Release body matches the `## [<version>]` CHANGELOG section |

## 3. No longer human — automated by T-107 through T-116

These cases need **no** human pass; do not re-run them by hand.

| Case | Automated by |
|---|---|
| T1.1 — `init` permissions and idempotence | T-107 |
| T1.6 — post-run resource hygiene | T-116 |
| T2.10 — `--pre` / `--verify` hook working directory and failure outcome | T-115 |
| T3.1, T3.6, T3.7 — egress allowlist composition | T-111 |
| T3.3, T3.4 — proxy blocked-destination UX and zero-denied promotability | T-112 |
| T4.8, T4.9, T4.12 — agent auth preflight failures | T-108 |
| T4.14, T4.15 — missing and malformed structured evidence at promote | T-109 |
| T4.19, T4.20 — run-ref resolution errors | T-110 |
| T4.22 — interrupt-driven cleanup | T-113 |
| T4.23 — binary file changes in diff artifacts | T-114 |

Two of these had stale expectations in the v0.1.0 manual plan text; the automated
tests are authoritative. T3.6 rejects `--egress open` combined with
`--egress-allow` at validation rather than warning and ignoring it: an allowlist
is meaningless without the proxy, so Tessariq fails closed and steers the operator
toward proxy mode. The spec (`specs/tessariq-v0.1.0.md#networking-and-egress`) does
not require warn-and-ignore, so rejection is the correct reading;
`TestE2E_EgressOpenWithAllowRejected` and `TestConfig_Validate_EgressOpenWithAllow`
are authoritative for the exact error. T1.6 post-run hygiene expectations differ
for retained worktrees (tracked by T-120).

## 4. Automated earlier

Covered by the suites before T-107 and likewise needing no human pass:
T1.4, T2.8, T3.5, T4.1, T4.3, T4.4, T4.6, T4.7, T4.11, T4.13, T4.16, T4.18, T4.21.

## 5. Known automation gaps

Mechanically automatable but currently thin: follow-up test work, **not**
human-only cases. A release does not block on them, but a spot-check is cheap
while running section 1.

| Case | Gap |
|---|---|
| T2.4 | The optional-config warning is emitted but asserted by no test, and it names the agent rather than the unmountable path the manual plan expects |
| T2.7 | No end-to-end proof of a *successful* agent update, or that the resulting version is at least the baked one; only the `--no-update-agent` and update-failure paths are covered |
| T4.2 | Task-path rejection is unit tested only; no e2e invokes `tessariq run` with a path outside the repository |
| T4.5 | Covers `docker` absent from `PATH`, not a running daemon that is stopped |
| T4.17 | Manifest identity tampering is covered for `run_id`; `task_path`, `base_sha` and task-title trailer fields are not validated |
| T4.24 | `version` and `--version` agree, but neither is exercised from a non-git directory |
