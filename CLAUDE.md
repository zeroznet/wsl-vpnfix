<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# CLAUDE.md: wsl-vpnfix

This file gives Claude Code project-specific context for working in this repo. Workspace-wide rules in `/home/zero/dev/CLAUDE.md` (Boba Bott persona, attribution header, commit format, behavioral guidelines, style guide) still apply; this file only adds what is unique to wsl-vpnfix.

## Session bootstrap

Before proposing any work or answering "what's next," read in this order:

1. `TODO.md` — open work in priority order (Now / Later / Backlog buckets)
2. `git log --oneline -20` — recent evolution
3. `cmd/wsl-vpnfix/main.go` and `internal/*/` — the actual runtime, single source of truth on behavior
4. `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` — living master design contract (last rebased 2026-05-10 to fold the Phase B addendum and Phase C decisions)
5. `docs/superpowers/plans/2026-05-08-wsl-vpnfix-phase-a-core-runtime.md` — frozen Phase A history (Corrections C-1..C-8 in the Self-Review section)
6. `docs/superpowers/plans/2026-05-09-wsl-vpnfix-phase-b-rootfs-and-release.md` — frozen Phase B history (rootfs, deterministic build, CI + release pipelines, Renovate, branch protection)
7. `docs/smoke-*.md` — real-host smoke evidence, one file per release event (newest: `docs/smoke-2026-11-18-v0.3.0.md`; latest VPN-bypass evidence: `docs/smoke-2026-10-04.md`)

Live files (TODO.md, code, current spec) win over frozen plans. Project memory at `~/.claude/projects/-home-zero-dev-wsl-vpnfix/memory/` is auto-loaded; `MEMORY.md` lives there as the index.

## What this is

A from-scratch rebuild of [`sakai135/wsl-vpnkit`](https://github.com/sakai135/wsl-vpnkit) (upstream last released `v0.4.1` on 2023-04-04 and has been dormant since). Same problem, repackaged: route WSL 2 traffic through the Windows host's network stack via [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock) so a Windows-side VPN doesn't black-hole WSL connectivity. No admin rights, no Windows-side config.

The fork exists to:

1. **Security-audit the original end-to-end** — shell script, Dockerfiles, distro `wsl.conf`, gvproxy invocation, the iptables NAT rules, the trap/cleanup paths, the `EUID` root check, the resolv.conf parsing, the interop assumption. Document what's fragile, what's actually a vulnerability, what's just sloppy.
2. **Replace the base images.** Upstream ships three Dockerfiles (`alpine`, `ubuntu`, `fedora`) with no version discipline. Build on **current Alpine only** (one base image is the smaller maintenance surface and the smaller audit surface; the runtime binary is identical regardless of base, so multi-base buys us nothing). Pin digest; rebuild on base updates.
3. **Repackage the runtime in Go.** Replace upstream's POSIX shell script with a single static Go binary. Eliminates shell-injection / trap-fragility / iptables-alias hacks. nftables driven via the netlink-typed Go library, not by shelling out. See `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` for the full architecture.
4. **Sleek public surface.** Same presentation discipline as [`zeroznet/nanocontext`](https://github.com/zeroznet/nanocontext): centered logo, badges, tight TOC, problem/flow/install/usage sections, prose that respects the reader. README is a product page, not a manual. Scope-fit principles (formerly a README "Anti-features" section, deleted 2026-05-11) are now enforced at submission time via the feature-request issue template's required checkboxes.

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
5. **Sleek README.** Logo, badges, TOC, problem statement, flow diagram, install matrix. Match nanocontext's discipline; do not copy its visual identity.

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
- **Pin third-party dependencies.** `containers/gvisor-tap-vsock` pinned to a release tag and commit in `build/upstream-pins.yaml`. `gvforwarder` is the upstream release binary, verified by SHA-256 before any unpack. `gvproxy-windowsgui.exe` is built from the pinned commit with `build/patches/gvproxy-hires-clock.patch` (F-019) until upstream ships an equivalent fix.
- **Distro-only install path.** No "standalone" mode. The user imports our pre-built rootfs via `wsl --import`. Single install path means single audit surface.
- **No new file without the attribution header** from `/home/zero/dev/CLAUDE.md` (Go files use `//`, shell uses `#`, markdown uses `<!-- ... -->`).

## What we explicitly do not do

- No Ubuntu or Fedora variants. Alpine only. Maintaining three base images was upstream's mistake, not ours.
- No POSIX shell runtime. Upstream's `wsl-vpnkit` shell script is the past; our orchestrator is Go.
- No standalone install path (drop binaries into the user's primary distro). Distro-only.
- No marketing fluff in the README. nanocontext's tone, not a startup landing page.
- No "improvements" outside the rebuild scope. Anything not on the goals list is a separate proposal.
- No code scanning workflow (CodeQL etc.). `govulncheck` + Dependabot cover the CVE surface; CodeQL adds CI + triage burden disproportionate at solo-project scale.

## Status

Phases A (Go orchestrator), B (rootfs, reproducible release pipeline, public surface) and C (threat model, security audit, master-spec rebase) are complete; their frozen plans are in `docs/superpowers/plans/`. Per-release history lives in git log, PR bodies and GitHub Releases; real-host evidence in `docs/smoke-*.md`.

**Current release: `v0.3.0`** as of 2026-11-18. Spurious-retransmit fix. Go's monotonic clock on Windows advances only per system tick (15.625 ms), gvproxy gave gVisor that clock, and on the ~1 ms bridge most RTT samples read 0, so RACK marked in-flight segments lost en masse (84% of received segments were duplicates). gvproxy.exe is now built from the pinned gvisor-tap-vsock v0.9.0 commit with `build/patches/gvproxy-hires-clock.patch` (QueryPerformanceCounter stack clock) in a new `gvproxy-builder` stage; gvforwarder stays the sha256-pinned upstream binary (PR #49, F-019, spec `docs/superpowers/specs/2026-11-18-gvproxy-hires-clock-design.md`). F-014 and F-015 closed as won't-fix. Smoke: `docs/smoke-2026-11-18-v0.3.0.md` (no VPN — 0 DSACK duplicates, throughput 9.1 → 17.9 MB/s).

**Out-of-band repo configuration** (applied via `gh api` / Settings, not in git): 10 topics; Wiki and Projects tabs disabled; Renovate labels `dependencies`, `deps:go`, `deps:alpine`, `deps:gvisor-tap-vsock`, `deps:github-actions`; Mend Renovate GitHub App installed 2026-07-28 with the dependency dashboard (Renovate bumps tags only, commit and sha256 pins stay manual, RR-D-1); `allow_auto_merge`, `delete_branch_on_merge`, `allow_update_branch` on; only merge commits allowed (squash and rebase off); tag ruleset on `v*` blocks deletion, non-fast-forward and update of release tags; Dependabot security updates and private vulnerability reporting on; social preview `assets/logo-social.png`. Branch protection and the merge sequence: project memory `project_pr_merge_dance_with_codex.md`.

## Repo layout

```
wsl-vpnfix/
├── CLAUDE.md                                       ← this file
├── README.md                                       ← public product page (nanocontext-style; centered logo, badges, TOC)
├── LICENSE                                         ← BSD-2-Clause
├── TODO.md                                         ← open work tracker (read this before starting any session)
├── renovate.json                                   ← 4 streams: gomod, alpine-3.24 digest + go-apk lockstep, gvisor-tap-vsock tag (sha256 manual, CI-guarded), github-actions
├── go.mod, go.sum                                  ← module github.com/zeroznet/wsl-vpnfix, go 1.26.8
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
│   ├── Dockerfile.rootfs                           ← four-stage: Go builder, gvproxy builder (pinned commit + patch), upstream fetcher (gvforwarder, SHA-verified), final Alpine assembly
│   ├── pack.sh                                     ← deterministic tarball producer (out/wsl-vpnfix-<version>.tar.gz; same commit -> same SHA)
│   ├── patches/gvproxy-hires-clock.patch           ← QueryPerformanceCounter clock for gvproxy's gVisor stack on Windows (F-019)
│   └── upstream-pins.yaml                          ← gvisor-tap-vsock release tag + commit + gvforwarder SHA-256
├── scripts/
│   └── install-wslvpnfix.ps1                       ← Windows-side PowerShell installer (download + SHA-verify + wsl --import + Task Scheduler At-logon)
├── dev/
│   ├── Containerfile                               ← Alpine 3.24 (digest-pinned) + Go 1.26.11 dev image
│   └── run.sh                                      ← podman wrapper with persistent caches; --integration adds NET_ADMIN+NET_RAW+/dev/net/tun
├── .github/workflows/
│   ├── ci.yml                                      ← gofmt, vet, mod-verify, govulncheck (non-blocking by design — apk go trails upstream), unit + integration, build verify, upstream-pins verify, release builder-stage verify, race
│   ├── release.yml                                 ← tag-triggered (^vN.N.N$); runs build/pack.sh; uploads tarball + SHA256SUMS + upstream-pins.yaml to GH Release
│   └── vulnwatch.yml                               ← weekly + toolchain PRs: strict govulncheck on the shipped apk go + Go EOL check; emails on failure
├── out/                                            ← gitignored; pack.sh write target
└── docs/
    ├── SECURITY-AUDIT.md                           ← per-finding audit catalogue (F-001..F-019)
    ├── THREAT-MODEL.md                             ← threat model
    ├── smoke-*.md                                  ← real-host smoke evidence, one file per release event
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
