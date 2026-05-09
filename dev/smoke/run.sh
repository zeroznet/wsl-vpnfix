#!/usr/bin/env sh
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)
#
# Foreground run of wsl-vpnfix for Phase A smoke test.
# Run as root after `dev/smoke/prepare.sh` succeeded.
# Leave running until Ctrl-C; verify in a second shell with `dev/smoke/verify.sh`.

set -eu

die() { printf '\033[1;31m==>\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" = "0" ] || die "must run as root: sudo dev/smoke/run.sh"
[ -x /sbin/wsl-vpnfix ]                    || die "/sbin/wsl-vpnfix not installed — run dev/smoke/prepare.sh first"
[ -x /sbin/wsl-gvforwarder ]               || die "/sbin/wsl-gvforwarder not installed — run dev/smoke/prepare.sh first"
[ -x /etc/wsl-vpnfix/wsl-gvproxy.exe ]     || die "/etc/wsl-vpnfix/wsl-gvproxy.exe not installed — run dev/smoke/prepare.sh first"

# -E preserves WSL_INTEROP, WSL_DISTRO_NAME, WSLENV across the sudo barrier.
# Without these, gvforwarder cannot spawn the Windows .exe via WSL interop.
exec sudo -E DEBUG=1 /sbin/wsl-vpnfix
