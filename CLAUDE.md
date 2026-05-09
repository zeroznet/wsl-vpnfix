<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# CLAUDE.md: wsl-vpnfix

This file gives Claude Code project-specific context for working in this repo. Workspace-wide rules in `/home/zero/dev/CLAUDE.md` (Boba Bott persona, attribution header, commit format, behavioral guidelines, style guide) still apply; this file only adds what is unique to wsl-vpnfix.

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

Repo has a design spec (`docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md`) and a Phase A implementation plan (`docs/superpowers/plans/2026-05-08-wsl-vpnfix-phase-a-core-runtime.md`). No code yet.

Phase A delivers the Go orchestrator, manually testable end-to-end on a developer's WSL 2 machine. Phase B delivers the rootfs assembly and reproducible build pipeline. Phase C delivers README, LICENSE, the PowerShell installer, and the initial security audit pass.

This file gets a real **Repo layout** table and a **Common commands** table once code exists. Until then, do not invent either.
