#!/usr/bin/env sh
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)
#
# Builds the wsl-vpnfix rootfs and packs it into a deterministic tar.gz at
# out/wsl-vpnfix-<version>.tar.gz, ready for `wsl --import`.
#
# Determinism inputs:
#   - SOURCE_DATE_EPOCH       (defaults to git commit time of HEAD)
#   - $VERSION                (positional arg or env)
#   - build/upstream-pins.yaml (pinned upstream artifact hashes)
#   - build/Dockerfile.rootfs  (digest-pinned Alpine base, apk versions)
#   - go.mod / go.sum          (locked Go module graph)
#
# Outputs:
#   out/wsl-vpnfix-<version>.tar.gz
#
# A clean rebuild from the same inputs above produces a bit-identical
# tarball SHA-256. CI in B7 enforces this for every release tag.

set -eu

log()   { printf 'pack: %s\n' "$*" >&2; }
warn()  { printf 'pack: warn: %s\n' "$*" >&2; }
die()   { printf 'pack: error: %s\n' "$*" >&2; exit 1; }
has()   { command -v "$1" >/dev/null 2>&1; }
need()  { has "$1" || die "missing required command: $1"; }

usage() {
    cat <<EOF
Usage: build/pack.sh <version>

Example: build/pack.sh 0.1.0
Produces: out/wsl-vpnfix-0.1.0.tar.gz
EOF
}

[ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ] && { usage; exit 0; }
VERSION="${1:-${VERSION:-}}"
[ -n "${VERSION}" ] || { usage; die "version required"; }

need git
need sha256sum
need awk
need tar

# Pick podman by default (rootless, what dev/run.sh already uses), fall
# back to docker if podman is missing. Same image build either way.
if has podman; then
    OCI=podman
elif has docker; then
    OCI=docker
else
    die "missing container runtime: podman or docker"
fi

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

COMMIT=$(git rev-parse --short=12 HEAD)
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git log -1 --pretty=%ct HEAD)}"
export SOURCE_DATE_EPOCH

# Parse upstream-pins.yaml without a YAML library — two lines, each is a
# `      sha256: <64hex>` with predictable indent. awk is sufficient and
# keeps the script dependency surface to POSIX tools.
PINS_FILE=build/upstream-pins.yaml
[ -f "${PINS_FILE}" ] || die "missing ${PINS_FILE}"

GVTV_TAG=$(awk '/^  tag:/ { print $2; exit }' "${PINS_FILE}")
[ -n "${GVTV_TAG}" ] || die "could not parse gvisor-tap-vsock tag from ${PINS_FILE}"

GVTV_GVFORWARDER_SHA256=$(awk '
    /^    gvforwarder:/ { in_gvf=1; next }
    in_gvf && /sha256:/ { print $2; exit }
' "${PINS_FILE}")
[ -n "${GVTV_GVFORWARDER_SHA256}" ] || die "could not parse gvforwarder sha256 from ${PINS_FILE}"

GVTV_GVPROXY_EXE_SHA256=$(awk '
    /^    gvproxy-windowsgui\.exe:/ { in_gvp=1; next }
    in_gvp && /sha256:/ { print $2; exit }
' "${PINS_FILE}")
[ -n "${GVTV_GVPROXY_EXE_SHA256}" ] || die "could not parse gvproxy-windowsgui.exe sha256 from ${PINS_FILE}"

# Refuse to ship if anything has not been filled in.
case "${GVTV_GVFORWARDER_SHA256}" in
    __FILL_FROM_UPSTREAM_SHA256SUMS__) die "${PINS_FILE}: gvforwarder sha256 placeholder, aborting" ;;
esac
case "${GVTV_GVPROXY_EXE_SHA256}" in
    __FILL_FROM_UPSTREAM_SHA256SUMS__) die "${PINS_FILE}: gvproxy-windowsgui.exe sha256 placeholder, aborting" ;;
esac

mkdir -p out
TAG_LOCAL="wsl-vpnfix-rootfs:${VERSION}"
TARFILE_RAW="out/wsl-vpnfix-${VERSION}.raw.tar"
TARFILE_FINAL="out/wsl-vpnfix-${VERSION}.tar.gz"
EXPORT_DIR="out/rootfs-${VERSION}"

log "building image ${TAG_LOCAL}"
${OCI} build \
    --file build/Dockerfile.rootfs \
    --tag  "${TAG_LOCAL}" \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "COMMIT=${COMMIT}" \
    --build-arg "SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}" \
    --build-arg "GVTV_TAG=${GVTV_TAG}" \
    --build-arg "GVTV_GVFORWARDER_SHA256=${GVTV_GVFORWARDER_SHA256}" \
    --build-arg "GVTV_GVPROXY_EXE_SHA256=${GVTV_GVPROXY_EXE_SHA256}" \
    .

log "exporting rootfs to ${EXPORT_DIR}"
rm -rf "${EXPORT_DIR}" "${TARFILE_RAW}" "${TARFILE_FINAL}"
mkdir -p "${EXPORT_DIR}"
CID=$(${OCI} create "${TAG_LOCAL}")
trap '${OCI} rm "${CID}" >/dev/null 2>&1 || true' EXIT
${OCI} export "${CID}" -o "${TARFILE_RAW}"

# Repack deterministically. We unpack into EXPORT_DIR, then re-tar with
# fixed mtime, sorted file order, owner 0:0, and consistent permissions.
log "repacking ${TARFILE_RAW} deterministically"
( cd "${EXPORT_DIR}" && tar -xf "../../${TARFILE_RAW}" )

# tar(1) flags for determinism: --sort=name (file order), --mtime= (fixed
# timestamp), --owner / --group / --numeric-owner (no host UID leak),
# --pax-option (drop variable-length extended headers).
( cd "${EXPORT_DIR}" && tar \
    --sort=name \
    --mtime="@${SOURCE_DATE_EPOCH}" \
    --owner=0 --group=0 --numeric-owner \
    --pax-option=exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime \
    -cf - . ) | gzip -n > "${TARFILE_FINAL}"

rm -f "${TARFILE_RAW}"
rm -rf "${EXPORT_DIR}"

SHA=$(sha256sum "${TARFILE_FINAL}" | awk '{print $1}')
SIZE=$(wc -c <"${TARFILE_FINAL}")
log "produced ${TARFILE_FINAL}"
log "  sha256: ${SHA}"
log "  bytes : ${SIZE}"
log "  inputs: tag=${GVTV_TAG} commit=${COMMIT} epoch=${SOURCE_DATE_EPOCH}"
