#!/usr/bin/env bash
# Pre-pull the third-party images a container-backed suite starts containers from.
#
# Usage: scripts/pull-test-images.sh <integration|e2e>
#
# Testcontainers pulls lazily, so a registry outage is paid once per container
# start: every test burns its own pull timeout before failing. Pulling up front
# with retries collapses that into one cheap step — a transient blip is retried
# instead of failing, and a real outage fails fast with one clear error instead
# of a full-length red suite.
#
# The lists are split per lane so neither suite waits on images it never starts.
# Only third-party images belong here. Images built during the run (the
# reference runtime layers, ghcr.io/tessariq/*, test images from
# testutil.BuildTestImage) are not pullable and are excluded on purpose.
set -euo pipefail

# Every lane starts these: the base image behind the testutil container helpers,
# Testcontainers' own reaper, and the host-side permission repair helper
# (internal/container.RepairImage) that workspace provisioning shells out to.
COMMON_IMAGES=(
  "alpine:latest"
  "testcontainers/ryuk:0.14.0"
  "alpine@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659"
)

# Integration-only: the git helper container, the reference runtime's build base
# (runtime/reference/Dockerfile), and the proxy tests' TLS origin and Squid bases.
INTEGRATION_IMAGES=(
  "alpine/git"
  "debian:bookworm-slim@sha256:f06537653ac770703bc45b4b113475bd402f451e85223f0f2837acbf89ab020a"
  "nginx:alpine"
  "ubuntu/squid:latest"
)

# E2e-only: the pinned proxy image (internal/proxy.DefaultSquidImage) that real
# `tessariq run --egress proxy` flows start.
E2E_IMAGES=(
  "ubuntu/squid@sha256:6a097f68bae708cedbabd6188d68c7e2e7a38cedd05a176e1cc0ba29e3bbe029"
)

lane="${1:-}"

case "$lane" in
  integration) IMAGES=("${COMMON_IMAGES[@]}" "${INTEGRATION_IMAGES[@]}") ;;
  e2e) IMAGES=("${COMMON_IMAGES[@]}" "${E2E_IMAGES[@]}") ;;
  *)
    echo "usage: $0 <integration|e2e>" >&2
    exit 2
    ;;
esac

readonly ATTEMPTS=3
readonly BACKOFF_SECONDS=5

failed=()

for image in "${IMAGES[@]}"; do
  attempt=1
  while true; do
    if docker pull --quiet "$image" >/dev/null; then
      echo "pulled $image"
      break
    fi

    if [ "$attempt" -ge "$ATTEMPTS" ]; then
      echo "pull failed after $ATTEMPTS attempts: $image" >&2
      failed+=("$image")
      break
    fi

    delay=$((BACKOFF_SECONDS * attempt))
    echo "pull failed (attempt $attempt/$ATTEMPTS), retrying in ${delay}s: $image" >&2
    sleep "$delay"
    attempt=$((attempt + 1))
  done
done

if [ "${#failed[@]}" -gt 0 ]; then
  echo "unable to pull ${#failed[@]} image(s): ${failed[*]}" >&2
  echo "The registry is unreachable; the $lane suite would fail on every container start." >&2
  exit 1
fi

echo "pulled ${#IMAGES[@]} $lane test image(s)"
