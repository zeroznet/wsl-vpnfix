#!/usr/bin/env sh
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)
#
# Run a command inside the wsl-vpnfix dev container with the project mounted
# at /work and persistent Go module / build caches under ~/.cache/wsl-vpnfix-dev.
#
# Usage:
#   dev/run.sh <cmd...>                  # build, unit tests, fmt, etc.
#   dev/run.sh --integration <cmd...>    # adds NET_ADMIN + NET_RAW + /dev/net/tun
#
# Examples:
#   dev/run.sh go test ./...
#   dev/run.sh go build -o /tmp/wsl-vpnfix ./cmd/wsl-vpnfix
#   dev/run.sh --integration go test -tags=integration ./internal/netlink/... -v

set -eu

PROJECT_DIR=$(cd "$(dirname "$0")/.." && pwd)
CACHE_DIR="${HOME}/.cache/wsl-vpnfix-dev"
IMAGE="localhost/wsl-vpnfix-dev:latest"

mkdir -p "${CACHE_DIR}/go" "${CACHE_DIR}/gocache"

INT_ARGS=""
if [ "${1:-}" = "--integration" ]; then
    INT_ARGS="--cap-add=NET_ADMIN --cap-add=NET_RAW --device=/dev/net/tun"
    shift
fi

if [ $# -eq 0 ]; then
    echo "usage: dev/run.sh [--integration] <cmd...>" >&2
    exit 2
fi

# shellcheck disable=SC2086
exec podman run --rm \
    -v "${PROJECT_DIR}:/work:Z" \
    -v "${CACHE_DIR}/go:/root/go:Z" \
    -v "${CACHE_DIR}/gocache:/root/.cache/go-build:Z" \
    -w /work \
    ${INT_ARGS} \
    "${IMAGE}" \
    sh -c "$*"
