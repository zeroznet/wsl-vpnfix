<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6) -->

# wsl-vpnfix — Security Audit

**Status:** Living catalogue, last reviewed 2026-05-10. Threat model: `docs/THREAT-MODEL.md`.

A short-catalogue audit of `wsl-vpnfix` covering the Phase A self-review corrections, Phase B follow-on fixes uncovered by smoke testing, the F-007 reversal, currently-tracked gaps, and tracked workarounds blocked on upstream movement. Format per `docs/THREAT-MODEL.md` section 5: one line per finding, severity implicit in the area grouping.

## Orchestrator

- F-001 TUNSETIFF MAC silently ignored — kernel auto-assigns random MAC, breaks gvproxy DHCP static lease lookup; status: fixed-in-v0.1.0 (Phase A C-1).
- F-002 pgroup signaling orphans gvproxy.exe grandchild — `cmd.Process.Signal(SIGTERM)` signals the gvforwarder leader only; `wsl-gvproxy.exe` inherits the pgroup but is not the leader so SIGTERM never reaches it; status: fixed-in-v0.1.0 (Phase A C-2).
- F-003 hardcoded eth0 fallback on route restore — `DelExistingDefaultRoute` deleted all IPv4 defaults and teardown restored via hardcoded `eth0`, silently breaking policy-routing users and non-eth0 NICs; status: fixed-in-v0.1.0 (Phase A C-3).
- F-004 absPathRe allows leading-dash exe path — path regex accepted `/foo/-bar` so an env-supplied path starting with `-` could reach a child `exec` as a flag, enabling argv smuggling; status: fixed-in-v0.1.0 (Phase A smaller corrections).
- F-005 user-edited resolv.conf silently misdirects NAT — if WSL did not auto-generate resolv.conf, the nameserver line was a user-supplied value and the parsed IP flowed into nftables DNAT without validation against the expected WSL2 gateway range; status: fixed-in-v0.1.0 (Phase A smaller corrections, autoGenMarker check + WSL2_GATEWAY_IP env override).
- F-006 whitespace-only env values pass validation — an env var set to spaces or tabs was not rejected, allowing a blank but non-empty string to reach path or IP validators; status: fixed-in-v0.1.0 (Phase A smaller corrections).
- F-008 pgroup-kill test assertion silently passes on zombie-state hosts — test would silently pass on PID-namespace hosts that do not reap orphans (rootless podman, busybox-init Alpine, etc.) because the grandchild lingers as a zombie and `kill(0)` returns nil instead of ESRCH; CI noise masks real pgroup-kill regressions; status: fixed-in-v0.1.0 (Phase A C-7 — accept ESRCH OR `/proc/<pid>/status State: Z` as proof of pgroup-kill success).
- F-010 gvproxy staged over 9P triggers EXCEPTION_IN_PAGE_ERROR — Win 11 25H2 demand-pages `.exe` code from `9P` (the Linux filesystem mount) which fails with `0xc0000006` before gvproxy.exe reads its first stdin byte; status: fixed-in-v0.1.0 (`974e9b4`, stages the exe to Windows NTFS via DrvFs so pages-in uses the native NTFS path).
- F-011 gvproxy default SSH listener conflicts on port 2222 — gvproxy v0.8.8 opens `127.0.0.1:2222` by default even when SSH forwarding is not needed; any pre-existing listener on that port causes gvproxy to exit with `cannot add network services`; status: fixed-in-v0.1.0 (`2b54529`, passes `-ssh-port=-1` to disable).

## Netfilter

- F-007 MASQUERADE saddr CIDR scope dropped sibling-distro packets — Phase A C-4 added `ip saddr 192.168.127.0/24` to the MASQUERADE rule per master spec section 3.5 example finding F-007, reasoning that unscoped masquerade could mask attacker-sourced packets; in production, sibling distros (Ubuntu, etc.) have eth0 IPs in `172.29.x.x`, outside `192.168.127.0/24`, so the qualifier caused their packets to bypass MASQUERADE, gvproxy saw the original `172.x.x.x` source IP, could not route the reply (its user-mode stack only knows `192.168.127.0/24`), and sibling connectivity failed entirely; the qualifier was the wrong fix for the actual functional requirement; status: reversed in `0893652` (Phase B B3 smoke evidence: docs/smoke-2026-05-10.md home-PC, docs/smoke-2026-05-10-workpc-vpn.md work-PC).
- F-009 nft atomic semantics abort on non-existent table delete — `Install` mixed `DelTable` + `AddTable` in one batched netlink transaction; deleting a non-existent table returns `ENOENT` which the batch propagates as an error, aborting the whole transaction and leaving no ruleset installed; status: fixed-in-v0.1.0 (Phase A C-5, split into two Conns: tolerant delete batch + strict create batch).

## WSL interop

- F-012 isDefaultDst nil check misses kernel-surfaced 0.0.0.0/0 routes — bare `r.Dst == nil` filtering in route capture missed default routes surfaced as `Dst=0.0.0.0/0` by some kernel versions, leaving the WSL2 NAT default in place at startup and defeating the tap redirect; status: fixed-in-v0.1.0 (Phase A C-6, `isDefaultDst` helper accepts both nil and 0.0.0.0/0).

## Build

- F-013 .gitignore bare entry swallowed cmd/wsl-vpnfix/ directory — bare `wsl-vpnfix` matched the `cmd/wsl-vpnfix/` directory subtree, silently eating untracked files inside it (`main_test.go` was lost on first `git add`); status: fixed-in-v0.1.0 (Phase A C-8, anchored to `/wsl-vpnfix`).

## Known gaps

- F-014 default-route persistence to disk for crash-safe recovery — orchestrator deletes the original WSL2 default route at init; a mid-init crash leaves sibling distros without networking until `wsl --shutdown` because the original routes are held only in process memory; status: tracked-in-TODO (added in Phase C C3 PR).
- F-015 no wsl-vpnfixctl debug subcommand — no built-in `status`, `dump-config`, or `verify-pins` command for operator diagnostics; failure diagnosis requires manual `nft list table wsl-vpnfix` + `ip route` + log inspection; status: tracked-in-TODO (added in Phase C C3 PR).

## Tracked workarounds

- F-016 govulncheck non-blocking in CI — Go 1.25.9 carries 2 stdlib CVEs (`GO-2026-4971` net/Dial NUL injection on Windows, `GO-2026-4918` net/http HTTP/2 SETTINGS_MAX_FRAME_SIZE infinite loop) fixed in Go 1.25.10; alpine apk pin blocks the bump until Alpine ships the updated package; both are low exposure for this threat model (Linux-only target, gvproxy-trusted HTTP/2 peer); CI step runs with `continue-on-error: true`; status: tracked-in-TODO (Backlog), re-enable strict govulncheck when alpine apk ships Go 1.25.10.
- F-017 gvproxy v0.8.8 stdio regression — upstream `cmd/gvproxy/config.go:125` parses `-listen-stdio` but never assigns it into `config.Interfaces.Stdio`; the stdio acceptor goroutine never starts, leaving the bridge TX-only; without the workaround, gvproxy.exe has no path back to gvforwarder and the whole bridge is silently dead; workaround stages a YAML config that sets `interfaces.stdio` directly, bypassing the broken CLI path; status: fixed-in-v0.1.0 (`e30d045`), re-evaluate at next gvisor-tap-vsock release.

## Re-audit triggers

See `docs/THREAT-MODEL.md` section 6.
