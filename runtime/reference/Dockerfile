FROM debian:bookworm-slim

# Baseline system packages (single layer).
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    jq \
    ripgrep \
    zip \
    unzip \
    tar \
    xz-utils \
    patch \
    procps \
    less \
    openssh-client \
    make \
    build-essential \
    pkg-config \
    python3 \
    python3-pip \
    python3-venv \
    gnupg \
  && rm -rf /var/lib/apt/lists/*

# Node LTS (22.x) via NodeSource.
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
  && apt-get install -y --no-install-recommends nodejs \
  && rm -rf /var/lib/apt/lists/* \
  && corepack enable

# Go 1.26.
RUN ARCH="$(dpkg --print-architecture)" \
  && curl -fsSL "https://go.dev/dl/go1.26.1.linux-${ARCH}.tar.gz" \
    -o /tmp/go.tar.gz \
  && tar -C /usr/local -xzf /tmp/go.tar.gz \
  && rm /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

# Non-root user.
RUN useradd --create-home --shell /bin/bash tessariq

USER tessariq
WORKDIR /home/tessariq
