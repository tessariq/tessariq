# Reference OpenCode agent image for quick onboarding and experimentation.
# Production users should build their own image; see docs/runtime-images.md.
FROM ghcr.io/tessariq/reference-runtime:v0.1.0

LABEL org.opencontainers.image.source="https://github.com/tessariq/tessariq"
LABEL org.opencontainers.image.description="Tessariq OpenCode agent runtime"
LABEL org.opencontainers.image.licenses="AGPL-3.0"

USER root
RUN npm install -g opencode-ai@1.3.15
USER tessariq
