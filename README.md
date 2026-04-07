<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo/lockup-horizontal-mono-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="assets/logo/lockup-horizontal-mono-light.svg">
  <img alt="Tessariq" src="assets/logo/lockup-horizontal-mono-dark.svg" height="64">
</picture>

[![CI](https://github.com/tessariq/tessariq/actions/workflows/ci.yml/badge.svg)](https://github.com/tessariq/tessariq/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/tessariq/tessariq)](https://github.com/tessariq/tessariq/blob/main/go.mod)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](https://github.com/tessariq/tessariq/blob/main/LICENSE)

# Agents run. Proof remains.

Tessariq is Git-native infrastructure for agent work. It runs coding agents in sandboxed, detached workspaces, captures durable evidence in open formats, and promotes results back into normal Git review only when you decide to.

The project is built around durable primitives: Git for history and review, and plain files like Markdown, JSON, YAML, and text for tasks, logs, manifests, and handoffs. Your repo stays yours. Your artifacts stay readable. Your agents stay replaceable.

## Why Tessariq

- Git-native: results flow back through the Git workflows teams already trust: branches, commits, diffs, and review.
- Detached by default: runs happen in isolated workspaces, so your active working tree is not silently mutated.
- Evidence-first: every run leaves behind inspectable artifacts you can read, debug, archive, or automate against.
- Safe by default: agent execution is containerized, egress can be controlled, and unsafe behavior requires explicit opt-in.

## What It Is

- A CLI for running coding agents against a repository in an isolated workspace.
- A detached workflow built around `run -> attach if needed -> promote`.
- An evidence model based on open, scriptable artifacts instead of opaque dashboards.
- A substrate for safe, reviewable agent execution that fits normal engineering practice.

## What It Is Not

- Not a proprietary agent IDE that owns your workflow.
- Not a one-workflow platform that traps context, artifacts, or review history.
- Not a claim of perfect determinism or autonomous infallibility.
- Not a web UI, database product, or multi-agent orchestration system today.

## Current Capabilities

The current CLI covers the core detached workflow:

- `tessariq init`: initialize repo-local runtime state under `.tessariq/`.
- `tessariq version`: print the CLI version.
- `tessariq run <task-path>`: run a task from a Markdown file in the current repository.
- `tessariq attach <run-ref>`: attach to a live run's `tmux` session.
- `tessariq promote <run-ref>`: promote a finished run into exactly one branch and one commit.

Current v0.1.0 scope:

- workspace mode: `worktree`
- agents: `claude-code`, `opencode`
- egress modes: `none`, `proxy`, `open`, `auto`
- runtime model: Docker image per run, with `--image` override support
- auth model: supported agent auth reused through read-only mounts
- promotion model: one reviewable Git commit, or a clean failure if there is no diff

## Prerequisites

To run Tessariq locally, you need:

- `git`
- `tmux`
- `docker`
- Go `1.26` if you want to build the CLI from source
- a compatible runtime image containing the selected agent binary
- supported local auth state for the selected agent

Tessariq checks required host prerequisites before it starts a run.

## Install

Build from source:

```sh
git clone https://github.com/tessariq/tessariq.git
cd tessariq
go install ./cmd/tessariq
tessariq version
tessariq --help
```

If you prefer a local binary in the repository directory:

```sh
go build ./cmd/tessariq
./tessariq version
./tessariq --help
```

## Prepare a Runtime Image

Tessariq runs agents inside Docker containers. The official reference runtime image includes a broad development toolchain, but it does not bundle third-party agent binaries. You derive your own image by adding the agent you want to run.

Example for Claude Code:

```dockerfile
FROM ghcr.io/tessariq/reference-runtime:v0.1.0

USER root
RUN npm install -g @anthropic-ai/claude-code@latest
USER tessariq
```

Build it:

```sh
docker build -t my-claude-runtime:v1 .
```

For more on runtime images, supported auth mounts, and OpenCode setup, see [`docs/runtime-images.md`](docs/runtime-images.md).

## Quickstart

Initialize repo-local runtime state:

```sh
tessariq init
```

Create a task file inside the repository:

```md
# Improve the README

Tighten the introduction, fix stale examples, and keep the tone technical and direct.
```

Assuming that file is saved as `tasks/improve-readme.md`, start a run:

```sh
tessariq run tasks/improve-readme.md --image my-claude-runtime:v1
```

Successful runs print script-friendly output:

```text
run_id: 01ARZ3NDEKTSV4RRFFQ69G5FAV
evidence_path: /path/to/repo/.tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV
workspace_path: /home/user/.tessariq/worktrees/repo-12345678/01ARZ3NDEKTSV4RRFFQ69G5FAV
container_name: tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV
attach: tessariq attach 01ARZ3NDEKTSV4RRFFQ69G5FAV
promote: tessariq promote 01ARZ3NDEKTSV4RRFFQ69G5FAV
```

Optional next steps:

```sh
tessariq attach last
tessariq promote last
```

Typical flow:

1. Write a task as Markdown inside the repo.
2. Run the task in an isolated workspace.
3. Inspect the evidence under `.tessariq/runs/<run_id>/`.
4. Attach if you want to observe or interact with the live session.
5. Promote the result into one branch and one commit when it is ready.

## What a Run Leaves Behind

Every run writes repo-local evidence under `.tessariq/runs/<run_id>/`.

```text
.tessariq/
  runs/
    index.jsonl
    <run_id>/
      manifest.json        # run metadata and resolved execution settings
      status.json          # lifecycle state, timing, exit code
      agent.json           # requested vs supported agent options
      runtime.json         # image identity and mount-policy metadata
      workspace.json       # workspace path, base SHA, reproducibility metadata
      task.md              # exact task file copied into evidence
      diff.patch           # patch output when the run changed code
      diffstat.txt         # change summary when the run changed code
      run.log              # captured agent output
      runner.log           # host-side runner and hook output
      egress.compiled.yaml # resolved allowlist in proxy mode
      egress.events.jsonl  # blocked egress attempts in proxy mode
```

These artifacts are plain files. No proprietary formats. No database required.

## Safety and Portability

- Dirty-repo guard: Tessariq refuses to start a run if the repository has staged, unstaged, or untracked non-ignored changes.
- Detached workspaces: v0.1.0 uses a detached Git worktree so `run` does not mutate your active working tree.
- Container isolation: agent execution happens in Docker, with capabilities dropped and privilege escalation disabled.
- Read-only auth reuse: supported auth state is mounted read-only; Tessariq does not expose host `HOME` inside the container.
- Controlled egress: proxy mode records resolved destinations and blocked egress attempts for auditability.
- Open artifacts: evidence stays in Markdown, JSON, YAML, and text, so it remains inspectable and portable outside any single product.

## Status

Tessariq is an in-progress open-source project with the current implementation centered on the first shippable release, `v0.1.0`.

- `v0.1.0` proves the detached local workflow: `run -> attach if needed -> promote`
- `v0.1.0` ships the `worktree` workspace model, evidence artifacts, runtime-image model, and proxy-based egress control
- `v0.2.0` expands workspace breadth with `copy+patch`, `repo-rw`, and same-mode `resume`
- later versions are expected to add operator-oriented commands such as `inspect`, `logs`, `list`, `stop`, `clean`, and `doctor`

## Read Next

- [`docs/runtime-images.md`](docs/runtime-images.md)
- [`specs/tessariq-v0.1.0.md`](specs/tessariq-v0.1.0.md)
- [`specs/tessariq-v0.2.0.md`](specs/tessariq-v0.2.0.md)
- [`specs/README.md`](specs/README.md)
- [`CHANGELOG.md`](CHANGELOG.md)

The versioned specs in `specs/` remain the normative source of truth for release scope and behavior.
