<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# CLAUDE.md: wsl-vpnfix

This file gives Claude Code project-specific context for working in this repo. Workspace-wide rules in `/home/zero/dev/CLAUDE.md` (Boba Bott persona, attribution header, commit format, behavioral guidelines #1–#12, style guide) still apply; this file only adds what is unique to wsl-vpnfix.

## Session bootstrap

Before proposing any work or answering "what's next," read in this order:

1. `TODO.md` — open work in priority order (Now / Later / Backlog buckets)
2. `git log --oneline -20` — recent evolution
3. `cmd/wsl-vpnfix/main.go` and `internal/*/` — the actual runtime, single source of truth on behavior
4. `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` — living master design contract (last rebased 2026-05-10 to fold the Phase B addendum and Phase C decisions)
5. `docs/superpowers/plans/2026-05-08-wsl-vpnfix-phase-a-core-runtime.md` — frozen Phase A history (Corrections C-1..C-8 in the Self-Review section)
6. `docs/superpowers/plans/2026-05-09-wsl-vpnfix-phase-b-rootfs-and-release.md` — frozen Phase B history (rootfs, deterministic build, CI + release pipelines, Renovate, branch protection)
7. `docs/smoke-2026-05-10.md` and `docs/smoke-2026-05-10-workpc-vpn.md` — production smoke evidence (gate results, MAC pinning, the F-007 reversal)

Live files (TODO.md, code, current spec) win over frozen plans. Project memory at `~/.claude/projects/-home-zero-dev-wsl-vpnfix/memory/` is auto-loaded; `MEMORY.md` lives there as the index.

## What this is

A from-scratch rebuild of [`sakai135/wsl-vpnkit`](https://github.com/sakai135/wsl-vpnkit) (upstream last released `v0.4.1` on 2023-04-04 and has been dormant since). Same problem, repackaged: route WSL 2 traffic through the Windows host's network stack via [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock) so a Windows-side VPN doesn't black-hole WSL connectivity. No admin rights, no Windows-side config.

The fork exists to:

1. **Security-audit the original end-to-end** — shell script, Dockerfiles, distro `wsl.conf`, gvproxy invocation, the iptables NAT rules, the trap/cleanup paths, the `EUID` root check, the resolv.conf parsing, the interop assumption. Document what's fragile, what's actually a vulnerability, what's just sloppy.
2. **Replace the base images.** Upstream ships three Dockerfiles (`alpine`, `ubuntu`, `fedora`) with no version discipline. Build on **current Alpine only** (one base image is the smaller maintenance surface and the smaller audit surface; the runtime binary is identical regardless of base, so multi-base buys us nothing). Pin digest; rebuild on base updates.
3. **Repackage the runtime in Go.** Replace upstream's POSIX shell script with a single static Go binary. Eliminates shell-injection / trap-fragility / iptables-alias hacks. nftables driven via the netlink-typed Go library, not by shelling out. See `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` for the full architecture.
4. **Sleek public surface.** Same presentation discipline as [`zeroznet/nanocontext`](https://github.com/zeroznet/nanocontext): centered logo, badges, tight TOC, problem/flow/install/usage/anti-features sections, prose that respects the reader. README is a product page, not a manual.

Project directory is `wsl-vpnfix`. The Git repo and any published image artifacts use the same name.

## Upstream architecture (what we're rebuilding)

Read this before changing anything in the runtime path. Every piece below is a candidate for redesign, not a contract.

```
Windows host                                 │ WSL 2 VM (wsl-vpnfix distro)
                                             │
  wsl-gvproxy.exe  ◀── stdio (interop) ───▶  │  wsl-vm  ──▶  tap dev (TAP_NAME)
  (gvisor-tap-vsock                          │              ├─ IP 192.168.127.2/24
   user-space network stack,                 │              ├─ MAC pinned
   talks to host network)                    │              └─ default route via .127.1
                                             │
                                             │  iptables (nat) on PREROUTING/OUTPUT:
                                             │   redirects WSL2 gateway IP → VPNKIT_HOST_IP
                                             │   redirects DNS (53/udp+tcp) → VPNKIT_GATEWAY_IP
                                             │   POSTROUTING MASQUERADE on tap
```

Key upstream pieces and what they do:

| File | Role | Notes for the rebuild |
|---|---|---|
| `wsl-vpnkit` (POSIX sh) | Sets up tap, runs `wsl-vm` + `wsl-gvproxy.exe` over stdio, installs NAT rules, runs connectivity checks, traps EXIT/INT/TERM to clean up. | Strict mode, no `set -x` over secrets, replace ad-hoc `alias iptables=iptables-legacy`, harden `EUID` check, validate env-var inputs (IPs, MACs, paths) before use. |
| `distro/{alpine,ubuntu,fedora}.dockerfile` | Build a minimal rootfs with the script, `wsl-vm`, `wsl-gvproxy.exe`, `wsl-vpnkit.service`, `wsl.conf`. | Pin base by digest. Audit installed packages. Drop fedora variant unless justified. |
| `distro/wsl.conf` | WSL boot config baked into the rootfs. | Verify `[boot]`, `[interop]`, `[automount]`, systemd settings; tighten if possible. |
| `wsl-vpnkit.service` | systemd unit invoking `wsl.exe -d wsl-vpnkit` (or the standalone script). | Replace permissive `Restart=always` + `KillMode=mixed` with sane defaults; consider `Type=notify`, sandboxing directives. |
| `build.sh` | `docker build` → `docker create` → `docker export | gzip` → `wsl-vpnkit.tar.gz`. | Replace `bash -xe` with strict POSIX, pinned digests, reproducible export, supply-chain notes (provenance, SBOM if cheap). |
| `import.sh` | Calls Windows `cmd.exe` to read `%USERPROFILE%`, then `wsl --unregister` + `wsl --import`. | Validate paths, fail loudly if `wsl.exe` is unavailable. |

External dependencies upstream pulls from GitHub release artifacts of `containers/gvisor-tap-vsock`: `gvproxy-windowsgui.exe` (renamed to `wsl-gvproxy.exe`) and `vm` (renamed to `wsl-vm`). Track these by version + checksum, not "latest".

## Goals (what done looks like)

1. **One canonical Alpine image.** Pinned by digest, reproducibly buildable on a clean machine.
2. **Audit document.** Short, honest, per-finding. Severity, file, line range, what an attacker or buggy interaction can do, fix or mitigation. Lives in `docs/SECURITY-AUDIT.md` (create only when there are findings to record).
3. **Single static Go binary as the runtime.** No shell. Strict input validation (regex per IP/MAC/path/hostname), explicit env allowlist for child processes, signal-driven lifecycle, idempotent cleanup that survives partial failures.
4. **Reproducible release pipeline.** Tagged release → built tarball + image digest + checksums + SBOM. No "build on my machine" releases.
5. **Sleek README.** Logo, badges, TOC, problem statement, flow diagram, install matrix, anti-features section. Match nanocontext's discipline; do not copy its visual identity.

## Source of truth references

| Need | Where |
|---|---|
| Upstream code, issues, history | `https://github.com/sakai135/wsl-vpnkit` |
| Networking backend | `https://github.com/containers/gvisor-tap-vsock` |
| WSL config docs | `https://learn.microsoft.com/en-us/windows/wsl/wsl-config` |
| WSL networking + interop | `https://learn.microsoft.com/en-us/windows/wsl/networking` |
| Presentation reference | `/home/zero/dev/nanocontext` (README, CLAUDE.md, repo layout) |

When the rebuild touches an external SDK or HTTP API, the workspace's `nanocontext` skill triggers automatically. Don't write integration code from training memory; let the cache fill first.

## Conventions specific to wsl-vpnfix

- **One base image: Alpine, pinned by digest** (`FROM alpine@sha256:...`). Update via PR with a one-line diff of why the new digest is safe. No Ubuntu, no Fedora — the runtime binary is identical regardless of base, multi-base buys us nothing but maintenance and audit surface.
- **Runtime is Go, not shell.** `cmd/wsl-vpnfix` + `internal/{config,netlink,netfilter,process,wsl,healthcheck}`. Strict mode in shell scripts (`#!/usr/bin/env sh`, `set -eu`) only applies to build scripts (`build/pack.sh`, `scripts/install-wslvpnfix.ps1`'s POSIX siblings). Application code is Go.
- **No shelling out from the Go binary.** No `os/exec` of `iptables`, `nft`, `ip`, `wsl.exe`, `cmd.exe`. nftables via netlink-typed library; tap/route/addr via netlink Go library; Windows .exe spawned by gvforwarder via its `stdio:` URL scheme, not by us directly.
- **Validate every env input.** IPs/MACs/paths/hostnames pass through strict regex validators before use. Reject path traversal (`..`) explicitly. No env value flows unvalidated into a syscall arg.
- **Explicit env allowlist for child processes.** When spawning `gvforwarder`, copy only `WSL_INTEROP`, `PATH`, `WSL_DISTRO_NAME`, `WSLENV` from the orchestrator's env. Empty `Env: []string{}` breaks WSL interop in some process tree shapes; full inherit gives audit surface we don't want.
- **Pin third-party binaries.** `gvforwarder` and `gvproxy-windowsgui.exe` from `containers/gvisor-tap-vsock` releases pinned to a release tag in `build/upstream-pins.yaml` and verified by SHA-256 before any unpack.
- **Distro-only install path.** No "standalone" mode. The user imports our pre-built rootfs via `wsl --import`. Single install path means single audit surface.
- **No new file without the attribution header** from `/home/zero/dev/CLAUDE.md` (Go files use `//`, shell uses `#`, markdown uses `<!-- ... -->`).

## What we explicitly do not do

- No Ubuntu or Fedora variants. Alpine only. Maintaining three base images was upstream's mistake, not ours.
- No POSIX shell runtime. Upstream's `wsl-vpnkit` shell script is the past; our orchestrator is Go.
- No standalone install path (drop binaries into the user's primary distro). Distro-only.
- No marketing fluff in the README. nanocontext's tone, not a startup landing page.
- No "improvements" outside the rebuild scope. Anything not on the goals list is a separate proposal.

## Status

**Phase A complete** as of 2026-05-09. Go orchestrator implemented per `docs/superpowers/plans/2026-05-08-wsl-vpnfix-phase-a-core-runtime.md`. All unit + integration tests green inside the dev container, race detector clean, `CGO_ENABLED=0` static binary builds bit-for-bit reproducibly.

**Phase B complete** as of 2026-05-10. Rootfs + reproducible release pipeline + public surface shipped per `docs/superpowers/plans/2026-05-09-wsl-vpnfix-phase-b-rootfs-and-release.md` (Phase B addendum has since been folded into the master spec by the Phase C rebase on 2026-05-10 and deleted; `git show 590d6cc:docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md` recovers it). Public repo at `github.com/zeroznet/wsl-vpnfix`; `v0.1.0` released with deterministic tarball + `SHA256SUMS` + `upstream-pins.yaml`; GitHub Actions CI (gofmt / vet / mod-verify / govulncheck non-blocking / unit + integration / build verify / race) and release pipeline (tag-triggered `^vN.N.N$`, runs `build/pack.sh`, uploads to GH Release); Renovate config (gomod + alpine-and-go-apk lockstep + gvisor-tap-vsock); branch protection on `main` (`enforce_admins`, required `ci` status check with `strict`, conversation resolution, no force-push, no delete). Public surface: README in nanocontext style, BSD-2-Clause LICENSE, `scripts/install-wslvpnfix.ps1` (PowerShell installer with per-user Task Scheduler At-logon auto-start, no admin). Smoke evidence: `docs/smoke-2026-05-10.md` (home PC, no VPN — bridge correctness) and `docs/smoke-2026-05-10-workpc-vpn.md` (work PC, Cisco AnyConnect cycled — the actual VPN-bypass property proven).

**Phase C complete** as of 2026-05-10. Three docs-only PRs landed per `docs/superpowers/plans/2026-05-10-wsl-vpnfix-phase-c-audit-and-release.md`: `docs/THREAT-MODEL.md` (PR #10, lifted from master spec section 3; master spec section 3 reduced to a one-line pointer); `docs/SECURITY-AUDIT.md` (PR #12, 17 findings grouped by area, severity implicit in grouping); master-spec rebase (PR #13, folds Phase B addendum into sections 4/6/8/frontmatter, drops v1.0 audit gate per C-D-1, deletes addendum file — recoverable via `git show 590d6cc:docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md`). `v0.2.0` tagged on `main`; release pipeline produced `wsl-vpnfix-0.2.0.tar.gz` + `SHA256SUMS` + `upstream-pins.yaml`; binary identical to v0.1.0 except for `-X main.version=v0.2.0` baked in. Two items redistributed to TODO Backlog: default-route persistence to disk for crash-safe recovery (cross-references audit finding F-014 — orchestrator holds the original WSL2 default route only in process memory; mid-init crash requires `wsl --shutdown` to recover) and `wsl-vpnfixctl` debug subcommand (cross-references audit finding F-015 — no built-in `status`/`dump-config`/`verify-pins` for operator diagnostics). One item dropped: fault-injection integration tests (cost of a netlink/netfilter mocking harness or privileged failure-injection harness exceeds the value for a single-user appliance whose only recovery path is `wsl --shutdown`). CI follow-on still tracked: re-enable strict `govulncheck` once alpine apk ships Go 1.25.10 (currently 2 stdlib CVEs flagged non-blocking — low exposure for this threat model).

**Current release: `v0.2.1`** as of 2026-05-11. Post-Phase-C patch: GitHub Actions bumped (`actions/checkout` v4→v6, `actions/setup-go` v5→v6) with `github-actions` added to Renovate (PR #17); README install example v0.1.0→v0.2.0 + Phase C spec frontmatter Status Draft→Completed + `/ica` architectural-review pass added to TODO Later (PR #18); README humanized (most em-dashes removed, "Anti-features" section dropped, VPN-impact sentence simplified) and installer fixed (wrapped in try/catch with throw-based `Die` so `iwr | iex` user-cancel or any error no longer kills the host shell; em-dash dropped from "Optional verify" line that rendered as `???` in Windows cp1252 console; Uninstall block colored consistent with Cyan section header + Green commands per `Write-Step` / `Write-Ok` style) (PR #19). Binary identical to v0.2.0 except for `-X main.version=v0.2.1`. Smoke verified live on workpc 2026-05-11 under Cisco AnyConnect: `wsl -d Debian -- curl -sI https://1.1.1.1` returned `HTTP/2 301` from Cloudflare (`cf-ray: 9f9f40184fd3a5dc-PRG`), proving the VPN-bypass property remains intact across the patch.

## Repo layout

```
wsl-vpnfix/
├── CLAUDE.md                                       ← this file
├── README.md                                       ← public product page (nanocontext-style; centered logo, badges, TOC)
├── LICENSE                                         ← BSD-2-Clause
├── TODO.md                                         ← open work tracker (read this before starting any session)
├── renovate.json                                   ← 3 streams: gomod, alpine+go-apk lockstep, gvisor-tap-vsock release
├── go.mod, go.sum                                  ← module github.com/zeroznet/wsl-vpnfix, go 1.25.9
├── cmd/wsl-vpnfix/                                 ← orchestrator main + buildEnv test
├── internal/
│   ├── config/                                     ← Config struct, validators, env loader
│   ├── healthcheck/                                ← HTTP(S) GET + DNS resolve probes
│   ├── netfilter/                                  ← nftables rule construction + install/remove via google/nftables
│   ├── netlink/                                    ← tap, addr, route via vishvananda/netlink
│   ├── process/                                    ← child-process manager (Setpgid, kill -pgid, WaitDelay 5s)
│   └── wsl/                                        ← WSL2 NAT gateway IP autodetect from resolv.conf
├── assets/
│   └── logo.svg                                    ← README banner (terminal mockup, GitHub-dark palette)
├── build/
│   ├── Dockerfile.rootfs                           ← three-stage: Go builder, upstream fetcher (SHA-verified), final Alpine assembly
│   ├── pack.sh                                     ← deterministic tarball producer (out/wsl-vpnfix-<version>.tar.gz; same commit -> same SHA)
│   └── upstream-pins.yaml                          ← gvisor-tap-vsock release tag + SHA-256s for gvforwarder + gvproxy.exe
├── scripts/
│   └── install-wslvpnfix.ps1                       ← Windows-side PowerShell installer (download + SHA-verify + wsl --import + Task Scheduler At-logon)
├── dev/
│   ├── Containerfile                               ← Alpine 3.23.4 (digest-pinned) + Go 1.25.9 dev image
│   └── run.sh                                      ← podman wrapper with persistent caches; --integration adds NET_ADMIN+NET_RAW+/dev/net/tun
├── .github/workflows/
│   ├── ci.yml                                      ← gofmt, vet, mod-verify, govulncheck (non-blocking pending alpine apk go 1.25.10), unit + integration, build verify, race
│   └── release.yml                                 ← tag-triggered (^vN.N.N$); runs build/pack.sh; uploads tarball + SHA256SUMS + upstream-pins.yaml to GH Release
├── out/                                            ← gitignored; pack.sh write target
└── docs/
    ├── smoke-2026-05-10.md                         ← home-PC bridge correctness validation
    ├── smoke-2026-05-10-workpc-vpn.md              ← work-PC + Cisco AnyConnect VPN-bypass validation
    └── superpowers/{specs,plans}/...               ← design contracts + phase plans (frozen-when-dated)
```

## Common commands

```sh
./dev/run.sh 'go test ./...'                                                                 # unit tests
./dev/run.sh --integration 'go test -tags=integration ./...'                                  # integration (root + caps)
./dev/run.sh 'CGO_ENABLED=1 go test -race -count=1 ./...'                                     # race detector (CGO required for -race)
./dev/run.sh 'gofmt -l . && go vet ./... && go vet -tags=integration ./...'                   # lint pass
./dev/run.sh 'CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -buildid=" -o /tmp/wsl-vpnfix ./cmd/wsl-vpnfix'   # production build (reproducible)
```
