<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix Phase B B4 — Smoke Notes (work PC, Cisco AnyConnect)

**Date:** 2026-05-10
**Install path:** Public PowerShell one-liner (regular PS, no admin) — `iwr -UseBasicParsing https://raw.githubusercontent.com/zeroznet/wsl-vpnfix/main/scripts/install-wslvpnfix.ps1 | iex`
**Release tarball:** `wsl-vpnfix-0.1.0.tar.gz` from `v0.1.0` GitHub Release
**Release tarball SHA-256:** `15c391f6a321b78eaa927d99bc6e8d79805770ce95bd3b0f84206a5453e33c9d`
**Built from commit:** `bfe44b8` (public-surface bundle; binary identical in function to home-PC smoke build at `0893652641c4` — only diff is README/logo/install.ps1/workflows/renovate, none of which are inside the rootfs)
**Host:** corporate Windows 11 (asset tag redacted), Cisco AnyConnect / Secure Client active during the run
**Sibling distro for verification:** Ubuntu
**Corp VPN:** **CONNECTED**, cycled up/down repeatedly during the test — this is the run that validates the actual project value proposition (route WSL traffic when a Windows-side VPN black-holes WSL connectivity).

## Result

GREEN on every gate. The VPN-bypass property is proven end-to-end on a real corp-VPN host. Repeated VPN client up/down cycling did not destabilize the bridge — orchestrator stayed alive, `wsl-gvproxy.exe` stayed alive, sibling distro traffic stayed available throughout.

| Gate | Result |
|---|---|
| 1. PowerShell one-liner pulls release tarball + verifies SHA-256 + imports as `wsl-vpnfix` distro | PASS — released SHA `15c391f6…` matched `SHA256SUMS` in the same release; `wsl --import` returned 0. |
| 2. Per-user Task Scheduler entry `wsl-vpnfix` registered (At-logon, hidden, RunLevel Limited, no admin) | PASS — `Get-ScheduledTask -TaskName wsl-vpnfix` shows `State=Ready`. |
| 3. Initial silent start triggered by `Start-ScheduledTask` — no root console window appears | PASS. |
| 4. Orchestrator alive in the appliance | PASS — `pgrep -af /sbin/wsl-vpnfix` returns one process at PID 7, child of `/init`. |
| 5. `wsltap` interface UP, MAC pinned, IP set, default route via gvproxy | PASS — `wsltap` UP, MAC `5a:94:ef:e4:0c:ee` (matches home-PC smoke), `192.168.127.2/24`, default via `192.168.127.1`. |
| 6. nftables `wsl-vpnfix` table installed with the post-F-007-reversal ruleset | PASS — `prerouting`/`output` DNAT both 10.255.255.254 → gvproxy gateway; `postrouting masquerade` is **unscoped** (no `ip saddr 192.168.127.0/24` qualifier), confirming the F-007 reversal is necessary in production for sibling-distro routing. |
| 7. Stdio bridge two-way under live traffic | PASS — `ip -s link show wsltap` reports RX 45 369 267 B / 32 220 packets and TX 1 827 855 B / 25 217 packets after a short browsing session, asymmetry consistent with HTTPS response/request shape. |
| 8. Sibling distro routes through us with corp VPN connected | PASS — `wsl -d Ubuntu -- curl -sI https://www.google.com` returns `HTTP/2 200` (DNS + TCP + TLS + HTTP/2 all green in one shot). |
| 9. Bridge survives Cisco AnyConnect cycling | PASS — VPN client toggled connect/disconnect multiple times during the test; orchestrator did not exit, `wsl-gvproxy.exe` did not exit, sibling-distro outbound traffic remained available across every transition. |

## Master-spec implications (work-PC validations)

Same as the home-PC note: section 3.5's example finding F-007 is **explicitly not present** in the production ruleset (the unscoped `postrouting masquerade oifname "wsltap"` is what makes sibling-distro routing work because their `eth0` IPs sit in `172.x.x.x`, outside `192.168.127.0/24`). Phase C `docs/SECURITY-AUDIT.md` should record this as a documented design decision, with the home-PC and work-PC smoke notes plus commit `0893652` as the evidence trail.

## Known not-yet-validated

- **Full Windows reboot → Task Scheduler At-logon trigger fires → appliance auto-starts → sibling traffic restored without manual action.** The chain is theoretically correct (per-user task with `-AtLogOn -User <self>` trigger, `Hidden` settings, hidden-PowerShell action), but the reboot was not exercised in this run. Low-risk gap — the trigger semantics are native Windows, not custom.
- **Long-running TCP across VPN flap (e.g. an `ssh` session held open while the VPN cycles).** Not exercised. Likely depends on TCP keepalive settings rather than anything wsl-vpnfix does.

## Conclusion

**Phase B B4 fully green.** Home-PC smoke (2026-05-10, no VPN) proved the bridge is technically correct; this work-PC smoke (2026-05-10, Cisco AnyConnect cycling) proves the actual VPN-bypass property. Phase B is closed. Phase C (SECURITY-AUDIT.md, THREAT-MODEL.md, master-spec rebase, `v1.0.0` tag) is the next milestone.
