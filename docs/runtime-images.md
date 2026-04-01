# Runtime Images

## Overview

Tessariq runs AI coding agents inside Docker containers. Each `tessariq run` invocation uses a **runtime image** that provides the operating system, development tools, and agent binary. The runtime image is the foundation of the sandboxed environment.

Tessariq ships one official **reference runtime image** that includes a broad development toolchain but does not bundle any third-party agent binary. Users derive their own images by adding the agent of their choice.

## Reference Runtime Image

The official reference runtime image for v0.1.0:

- **Image**: `ghcr.io/tessariq/reference-runtime:v0.1.0`
- **Base**: `debian:bookworm-slim` (glibc-based)
- **Default user**: `tessariq` (non-root)
- **Tags**: versioned only (`v0.1.0`); there is no `:latest` tag contract

### Baseline toolchain

The reference runtime includes:

| Category | Packages |
|----------|----------|
| Shell and core | `bash`, `ca-certificates`, `curl`, `git`, `jq`, `ripgrep` |
| Archive and patch | `zip`, `unzip`, `tar`, `xz-utils`, `patch` |
| System | `procps`, `less`, `openssh-client` |
| Build | `make`, `build-essential`, `pkg-config` |
| Python | Python 3 with `pip` and `venv` |
| Node | Node.js LTS (22.x) with `npm` and `corepack` |
| Go | Go 1.26 |

The reference runtime does **not** bundle Claude Code, OpenCode, or any other third-party agent binary.

## Auth State Reuse

Tessariq does **not** mount the host-installed agent binary into the container. The agent binary must already exist inside the runtime image.

Tessariq **does** reuse supported auth state by mounting credential files read-only into the container:

| Agent | Auth files mounted |
|-------|-------------------|
| Claude Code | `~/.claude/.credentials.json`, `~/.claude.json` |
| OpenCode | `~/.local/share/opencode/auth.json` |

This means you install the agent once in your image, and Tessariq handles credential forwarding.

## Deriving a Compatible Runtime Image

Since the reference runtime does not include agent binaries, you need to derive a new image that adds the agent you want to use. A compatible runtime image must:

- Use a glibc-based Linux base image
- Have a non-root default user
- Have the agent binary available in `PATH`

### Example: Adding Claude Code

```dockerfile
FROM ghcr.io/tessariq/reference-runtime:v0.1.0

USER root
RUN npm install -g @anthropic-ai/claude-code@latest
USER tessariq
```

Build and use:

```sh
docker build -t my-claude-runtime:v1 .
tessariq run tasks/fix-login-bug.md --image my-claude-runtime:v1
```

`<task-path>` must be a Markdown file inside the current repository.

### Example: Adding OpenCode

```dockerfile
FROM ghcr.io/tessariq/reference-runtime:v0.1.0

USER root
RUN curl -fsSL https://opencode.ai/install.sh | sh
USER tessariq
```

## Using a Custom Runtime Image

Override the runtime image for any run with `--image`:

```sh
tessariq run tasks/implement-feature-x.md --image my-registry/my-runtime:v1
```

The selected agent binary must be present in the specified image.

## Future: macOS Keychain Host Helper (Informative)

Direct Claude Code Keychain reuse is **not** part of the v0.1.0 contract. A future host-helper approach on macOS could extract credentials from the system Keychain into a short-lived temp file:

```sh
#!/bin/sh
set -eu

tmp="$(mktemp)"
chmod 600 "$tmp"

security find-generic-password -a "$USER" -s "Claude Code-credentials" -w > "$tmp"

printf '%s\n' "$tmp"
```

The intended future flow:

1. Run the helper on the macOS host.
2. Write a short-lived temp credentials file with mode `0600`.
3. Mount that file read-only into the container at `$HOME/.claude/.credentials.json`.
4. Delete the temp file after the run completes.

This sketch is informative only. It must not use `CLAUDE_CODE_OAUTH_TOKEN` because of known upstream macOS side effects around Keychain state.
