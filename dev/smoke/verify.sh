#!/usr/bin/env sh
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)
#
# Phase A smoke-test verifier. Auto-detects mode from the kernel state:
#   - wsltap UP                                  → live mode (run while smoke/run.sh is active)
#   - no wsltap, no wsl-vpnfix nft table         → teardown mode (run after Ctrl-C)
# Each mode runs its checks and prints PASS / FAIL per check plus a summary.

set -u

GREEN='\033[1;32m'
RED='\033[1;31m'
YEL='\033[1;33m'
RST='\033[0m'
PASS=0
FAIL=0
CHECK_HOST="example.com"

ok()    { printf "${GREEN}PASS${RST} %s\n"  "$*"; PASS=$((PASS+1)); }
bad()   { printf "${RED}FAIL${RST} %s\n"    "$*"; FAIL=$((FAIL+1)); }
note()  { printf "${YEL}NOTE${RST} %s\n"    "$*"; }
has_cmd() { command -v "$1" >/dev/null 2>&1; }

has_cmd ip   || { printf "iproute2 not installed\n" >&2; exit 2; }
has_cmd nft  || { printf "nftables CLI not installed\n" >&2; exit 2; }

if ip link show wsltap >/dev/null 2>&1; then
    MODE=live
else
    MODE=teardown
fi

printf "${YEL}==>${RST} mode: %s\n\n" "$MODE"

case "$MODE" in
live)
    # 1. wsltap is up
    if ip link show wsltap | grep -q 'state UP\|<.*UP.*>'; then
        ok "wsltap interface UP"
    else
        bad "wsltap interface present but not UP"
    fi

    # 2. nft table installed
    if nft list table ip wsl-vpnfix >/dev/null 2>&1; then
        ok "nftables table 'wsl-vpnfix' present"
    else
        bad "nftables table 'wsl-vpnfix' missing"
    fi

    # 3. default route via wsltap
    if ip route get 1.1.1.1 2>/dev/null | grep -q wsltap; then
        ok "default route to 1.1.1.1 via wsltap"
    else
        bad "default route to 1.1.1.1 NOT via wsltap"
        ip route get 1.1.1.1 2>&1 | sed 's/^/     /'
    fi

    # 4. DNS resolves
    if has_cmd nslookup && nslookup "$CHECK_HOST" >/dev/null 2>&1; then
        ok "DNS resolves $CHECK_HOST"
    elif has_cmd getent && getent hosts "$CHECK_HOST" >/dev/null 2>&1; then
        ok "DNS resolves $CHECK_HOST (via getent)"
    else
        bad "DNS does NOT resolve $CHECK_HOST"
    fi

    # 5. HTTPS reaches example.com
    if has_cmd curl; then
        if curl -fsS -m 5 -o /dev/null -w '%{http_code}' "https://$CHECK_HOST" 2>/dev/null | grep -qE '^(200|301|302)$'; then
            ok "HTTPS GET https://$CHECK_HOST returned 2xx/3xx"
        else
            bad "HTTPS GET https://$CHECK_HOST failed"
        fi
    else
        note "curl not installed — skipping HTTPS check"
    fi
    ;;

teardown)
    # 1. wsltap is gone
    if ip link show wsltap >/dev/null 2>&1; then
        bad "wsltap still exists after teardown"
    else
        ok "wsltap removed"
    fi

    # 2. nft table is gone
    if nft list table ip wsl-vpnfix >/dev/null 2>&1; then
        bad "nftables table 'wsl-vpnfix' still present after teardown"
    else
        ok "nftables table 'wsl-vpnfix' removed"
    fi

    # 3. some default route exists (original WSL2 default restored, or whatever was there)
    if ip route show default 2>/dev/null | grep -q '^default'; then
        ok "default route present (likely original WSL2 default restored)"
    else
        bad "no default route present after teardown"
    fi

    # 4. wsl-vpnfix process is gone
    if pgrep -x wsl-vpnfix >/dev/null 2>&1; then
        bad "/sbin/wsl-vpnfix process still running"
    else
        ok "no wsl-vpnfix process running"
    fi

    # 5. gvforwarder is gone
    if pgrep -x wsl-gvforwarder >/dev/null 2>&1; then
        bad "/sbin/wsl-gvforwarder process still running"
    else
        ok "no wsl-gvforwarder process running"
    fi
    ;;
esac

printf "\n${YEL}==>${RST} %s mode result: %s pass, %s fail\n" "$MODE" "$PASS" "$FAIL"
[ "$FAIL" = "0" ]
