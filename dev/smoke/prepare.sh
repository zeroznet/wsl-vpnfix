#!/usr/bin/env sh
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)
#
# Idempotent: stage everything Phase A smoke-test needs on a WSL 2 distro.
#   - verify environment (WSL 2, root, Go)
#   - build /sbin/wsl-vpnfix from this repo
#   - download gvisor-tap-vsock v0.8.8 release artifacts
#   - verify upstream sha256sums
#   - install /sbin/wsl-gvforwarder and /etc/wsl-vpnfix/wsl-gvproxy.exe
#
# Run as root from the repo root: `sudo dev/smoke/prepare.sh`
# Re-run is safe — every step is idempotent.

set -eu

GVTV_TAG="v0.8.8"
GVTV_RELEASE_URL="https://github.com/containers/gvisor-tap-vsock/releases/download/${GVTV_TAG}"
WORK_DIR="${TMPDIR:-/tmp}/wsl-vpnfix-smoke"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

log()  { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m==>\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m==>\033[0m %s\n' "$*" >&2; exit 1; }
has_cmd() { command -v "$1" >/dev/null 2>&1; }
need_cmd() { has_cmd "$1" || die "Missing required command: $1"; }

### environment checks
[ "$(id -u)" = "0" ] || die "must run as root: sudo dev/smoke/prepare.sh"
grep -qiE "(microsoft|wsl)" /proc/version 2>/dev/null \
    || warn "this kernel does not look like WSL 2 (continuing anyway, but smoke test only validates on real WSL 2)"
need_cmd sha256sum
need_cmd install
has_cmd curl || has_cmd wget \
    || die "Missing required command: need either 'curl' or 'wget'"
has_cmd go \
    || die "Go toolchain not installed. Install with 'apt install golang-go' (Debian/Ubuntu) or 'apk add go' (Alpine), then re-run."

### build wsl-vpnfix
log "building /sbin/wsl-vpnfix from ${REPO_ROOT}"
cd "${REPO_ROOT}"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -buildid=" \
    -o "${WORK_DIR}/wsl-vpnfix" ./cmd/wsl-vpnfix \
    || die "go build failed"
install -m 0755 "${WORK_DIR}/wsl-vpnfix" /sbin/wsl-vpnfix
log "installed: $(/sbin/wsl-vpnfix --version)"

### stage upstream binaries
mkdir -p "${WORK_DIR}"
cd "${WORK_DIR}"

fetch() {
    if has_cmd curl; then
        curl -fSL --retry 3 -o "$2" "$1"
    else
        wget -q -O "$2" "$1"
    fi
}

log "fetching gvisor-tap-vsock ${GVTV_TAG} artifacts"
fetch "${GVTV_RELEASE_URL}/sha256sums"           sha256sums
fetch "${GVTV_RELEASE_URL}/gvforwarder"          gvforwarder
fetch "${GVTV_RELEASE_URL}/gvproxy-windowsgui.exe" gvproxy-windowsgui.exe

log "verifying upstream checksums"
grep -E '(gvforwarder|gvproxy-windowsgui\.exe)$' sha256sums > sha256sums.filtered \
    || die "upstream sha256sums file did not contain expected artifact lines"
sha256sum -c sha256sums.filtered \
    || die "upstream sha256sums verification failed — refusing to install"

log "installing /sbin/wsl-gvforwarder and /etc/wsl-vpnfix/wsl-gvproxy.exe"
install -m 0755 gvforwarder /sbin/wsl-gvforwarder
install -m 0755 -D gvproxy-windowsgui.exe /etc/wsl-vpnfix/wsl-gvproxy.exe

### final summary
log "ready. Phase A smoke test:"
printf '  1) shell A: sudo dev/smoke/run.sh\n'
printf '  2) shell B: dev/smoke/verify.sh   (while run.sh is running)\n'
printf '  3) shell A: Ctrl-C\n'
printf '  4) shell B: dev/smoke/verify.sh   (re-run after teardown)\n'
