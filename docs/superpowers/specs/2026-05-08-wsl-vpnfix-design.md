<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix — Design

**Date:** 2026-05-08
**Status:** Living spec, last rebased 2026-05-10 against Phase B addendum and Phase C decisions.
**Author:** Robert Bopko, with Claude Opus 4.7 assistance

A from-scratch rebuild of [`sakai135/wsl-vpnkit`](https://github.com/sakai135/wsl-vpnkit) (upstream dormant since 2023-04-04). Same problem class — bypass a Windows-side VPN that breaks WSL 2 networking — but built for 2026: minimal attack surface, single Go binary, signed reproducible releases, documented threat model.

---

## 1. Problem & target user

### 1.1 Problem

Some corporate VPN clients on Windows hosts (e.g. Cisco AnyConnect) break WSL 2 networking so badly that even `networkingMode=mirrored` (Win 11 22H2+) does not fix it. Typical offenders: VPNs with deep packet inspection, aggressive split-tunneling rules, custom DNS routing, or enforced full-tunnel that drops packets from the WSL VM as "uninstalled client".

### 1.2 Target user

Anyone on Windows 11 with WSL 2 who:

- has a VPN that mirrored mode can't satisfy,
- can't switch the VPN client (policy or otherwise),
- wants an auditable, signed, reproducibly-built tool instead of "download some random tarball from the internet".

### 1.3 Anti-target users

- Home VPNs / OpenVPN / WireGuard cases that mirrored already handles. Switch WSL to mirrored and skip us.
- People looking to *replace* a VPN client. This is not a VPN; it is a network bypass through the Windows host.
- Windows 10, macOS, native Linux. Strictly Windows 11 + WSL 2.

### 1.4 Value vs upstream

Same technical approach (gvisor-tap-vsock over a Win↔Linux stdio bridge), but:

- Go runtime instead of bash — eliminates shell-injection / trap fragility.
- Pin-and-verify supply chain (SHA-256 sourced from upstream's `sha256sums`).
- One image (Alpine) instead of three (alpine / ubuntu / fedora).
- One install path (distro-only) instead of two (distro + standalone).
- Documented and audited threat model.
- Active maintenance.

---

## 2. Architecture

### 2.1 Topology

```
Windows 11 host                          ┊  WSL 2 (single Linux VM, default NAT mode)
                                         ┊
  ┌─────────────────────┐                ┊   ┌──────────────────────────────────┐
  │ Corporate VPN client│                ┊   │ wsl-vpnfix distro (Alpine)       │
  │   (e.g. AnyConnect) │                ┊   │                                  │
  └──────────┬──────────┘                ┊   │  /sbin/wsl-vpnfix  (our Go bin)  │
             │                           ┊   │   ├─ validate env / config       │
  ┌──────────▼──────────┐                ┊   │   ├─ spawn gvforwarder ─────┐    │
  │ Windows host        │                ┊   │   ├─ install nft NAT rules  │    │
  │ network stack       │                ┊   │   ├─ create tap (wsltap)    │    │
  └──────────▲──────────┘                ┊   │   └─ wait + signal handler  │    │
             │ host syscalls (sockets)   ┊   │                              │    │
  ┌──────────┴──────────┐                ┊   │  /sbin/wsl-gvforwarder ◄────┘    │
  │ gvproxy-windowsgui  │ ◄── stdio ───┐ ┊   │   (pinned upstream binary)       │
  │ .exe (pinned)       │              │ ┊   │   reads frames from tap fd       │
  │ user-space TCP/IP   │              └─┊───┤   pipes them to gvproxy stdin    │
  │ from gvisor-tap-    │              ┄ ┊ ┄ │   reverse direction same          │
  │ vsock              │                ┊   │                                  │
  └─────────────────────┘                ┊   │  tap dev "wsltap"                │
   spawned via WSL interop               ┊   │   ├─ MAC = pinned                │
                                         ┊   │   ├─ IP  = 192.168.127.2/24      │
                                         ┊   │   └─ default route via .127.1    │
                                         ┊   └──────────────────────────────────┘
                                         ┊                ▲
                                         ┊                │ shared kernel netns
                                         ┊   ┌────────────┴─────────────────────┐
                                         ┊   │ User's Ubuntu / Debian / ...     │
                                         ┊   │ Their packets hit our nft NAT    │
                                         ┊   │ and get steered to wsltap        │
                                         ┊   └──────────────────────────────────┘
```

### 2.2 Why this works

WSL 2 (default NAT mode) hosts every imported distro inside one Linux VM with a shared kernel and shared network namespace. When `wsl-vpnfix` installs nftables NAT rules, those rules apply to traffic from every other distro in the same VM. That's the mechanism the original wsl-vpnkit relied on, and it still works.

`wsl-vpnfix` never replaces the user's primary distro. It runs as a sibling. The user keeps their Ubuntu / Debian / Arch and routes through us transparently.

### 2.3 Components

| Component | Where | Owner | Pinned? |
|---|---|---|---|
| `wsl-vpnfix` Go orchestrator | Linux side, in our distro, run by systemd | us | source-in-repo |
| `wsl-gvforwarder` | Linux side, child of orchestrator | upstream `containers/gvisor-tap-vsock` | yes (SHA-256) |
| `wsl-gvproxy.exe` | Windows side, child of orchestrator (WSL interop) | upstream `containers/gvisor-tap-vsock` (`gvproxy-windowsgui.exe`) | yes (SHA-256) |
| Alpine rootfs | Linux side, the distro we ship | us, base from Alpine | base image pinned by digest |
| `wsl-vpnfix.service` | Linux side, baked into rootfs | us | source-in-repo |
| `wsl.conf` | Linux side, baked into rootfs | us | source-in-repo |

### 2.4 Lifecycle

1. **Boot:** WSL launches the distro. systemd starts `wsl-vpnfix.service`.
2. **Init:** orchestrator reads env vars; validates every IP / MAC / path against strict regex; refuses to start on any malformed input.
3. **Spawn gvproxy on Windows:** orchestrator launches `wsl-gvproxy.exe` via WSL interop with stdio plumbed to a pipe pair owned by the orchestrator.
4. **Spawn gvforwarder on Linux:** with the same stdio pair, gvforwarder opens `/dev/net/tun`, configures `wsltap`, starts the frame copy loop.
5. **Network setup:** orchestrator brings `wsltap` up via netlink, assigns IP, installs default route, applies nft NAT rules through the netlink-typed nftables library (no shelling to `nft` or `iptables`).
6. **Health probes:** ping gateway, resolve a known DNS name, fetch a small HTTPS endpoint. Result is logged but does not gate startup.
7. **Steady state:** main goroutine blocks on `signal.Notify(SIGINT, SIGTERM)` and on each child's `cmd.Wait()` channel.
8. **Stop / crash:** any signal or child exit triggers ordered teardown — remove nft rules, take tap down, SIGTERM children, wait with timeout, SIGKILL on timeout, exit.

### 2.5 Configuration

- The IP plan (`192.168.127.0/24`, gateway `.1`, host `.254`, local `.2`) is **compile-time fixed**, not env-overridable. gvproxy v0.8.8 hardcodes the host IP (`.254`) in its DNS records for `host.containers.internal` and the `.2:5a:94:ef:e4:0c:ee` mapping in its DHCP static lease map (`cmd/gvproxy/config.go:22-23, 372`); those defaults apply in the CLI mode that gvforwarder spawns gvproxy in (no `-config` file). Changing the subnet at our orchestrator level would silently leave `host.containers.internal` pointing outside the new subnet — a pretend-knob, not a real one. Hardcoded constants in `internal/config` reflect this constraint honestly. Upstream wsl-vpnkit exposed the IPs as env vars but those overrides were equally pretend; we drop them.
- Other runtime knobs (WSL2 NAT gateway IP override, tap name and MAC, gvproxy/gvforwarder paths, healthcheck targets, debug flag) come from env vars consumed by the systemd unit (`Environment=`).
- Every env value is validated against a strict regex before use. No env value is concatenated into a syscall arg or passed to a child unmodified.
- A `--print-config` flag dumps the resolved configuration for debugging.

### 2.6 Rootfs contents

The orchestrator is launched at distro boot via `wsl.conf` `[boot] command=/sbin/wsl-vpnfix`, running as a child of WSL's own `/init` (not as PID 1). The `systemd=true` path was tested on 2026-05-09 and broke WSL interop's stdio bridge to gvproxy.exe due to systemd-style integration assumptions WSL makes that we cannot satisfy without shipping systemd; `command=` avoids that entirely. A `/sbin/init` symlink stays in the rootfs as defensive cover; it no-ops at runtime because `os.Getpid() != 1`.

```
/sbin/wsl-vpnfix              ← our Go binary (orchestrator)
/sbin/wsl-gvforwarder         ← pinned upstream Linux binary
/etc/wsl-vpnfix/wsl-gvproxy.exe
                              ← pinned upstream Windows binary, shipped here;
                                staged at boot to Windows NTFS before launch
/etc/wsl-vpnfix/checksums     ← SHA-256 of bundled binaries, verified at boot
/etc/wsl.conf                 ← [boot] command=/sbin/wsl-vpnfix; interop on; automount hardened; no systemd
/sbin/init                    ← symlink to /sbin/wsl-vpnfix (defensive cover, unused at runtime)
LICENSE
```

Plus busybox, Alpine baselayout, `nftables`, `iproute2`, `ca-certificates`. No `bash`, no `curl`, no `wget`, no compilers, no SSH.

---

## 3. Threat model & audit scope

See `docs/THREAT-MODEL.md`. Lifted from this section on 2026-05-10; this header survives only as a section number anchor for the rest of the document.

---

## 4. Build & release pipeline

### 4.1 Inputs

```
Source              ← this repo, git tag is the source of truth
Go toolchain        ← pinned in go.mod via `toolchain go1.25.0` (or current stable at v1.0; Go has no LTS, follow upstream stable)
Alpine rootfs base  ← FROM alpine@sha256:<digest>  (pinned, not :latest, not :3.22)
gvforwarder         ← https://github.com/containers/gvisor-tap-vsock/releases/download/<tag>/gvforwarder
gvproxy-windowsgui  ← https://github.com/containers/gvisor-tap-vsock/releases/download/<tag>/gvproxy-windowsgui.exe
sha256sums          ← https://github.com/containers/gvisor-tap-vsock/releases/download/<tag>/sha256sums
```

The upstream tag and our expected checksums for the two artifacts live in one repo file, `build/upstream-pins.yaml`:

```yaml
gvisor_tap_vsock:
  tag: v0.8.8
  artifacts:
    gvforwarder:            sha256:<64 hex>
    gvproxy-windowsgui.exe: sha256:<64 hex>
```

The build verifies upstream's `sha256sums` against these values before any unpack. Any drift fails the build loudly.

**Pinning policy — what we explicitly version-control vs. what falls out of the base:**

We pin exactly four things. Everything else in the build environment is downstream of those pins and bumps in lockstep when we bump them:

| Pinned thing | Where the pin lives | What it locks |
|---|---|---|
| Alpine base image | `FROM alpine@sha256:<digest>` in Containerfile / Dockerfile.rootfs | Every apk package in the resulting image (nftables, iproute2, ca-certificates, busybox, libc, etc.) — they are whatever ships in that Alpine snapshot. |
| Go toolchain | `go=<version>-r<rev>` in `apk add` of the builder image, plus `go <version>` directive in `go.mod` | The compiler / stdlib that produces the binary. Builder pin must match `go.mod` directive so a base bump cannot silently change the language version. |
| Go module dependencies | `go.sum` (verified at build time via `go mod verify` against `sum.golang.org`) | Every Go library compiled into the binary, including transitive deps. |
| gvisor-tap-vsock release | `build/upstream-pins.yaml` (tag + sha256 of each artifact) | The `gvforwarder` Linux binary and `gvproxy-windowsgui.exe` shipped in the rootfs. |

We **do not** pin individual apk packages by version (no `nftables=1.1.5-r2`, no `iproute2=6.17.0-r0`). The Alpine base digest already locks them; per-package pins would be redundant noise that drifts out of sync with reality every time the base bumps. An apk pin is justified only when version really matters independent of the base — Go is the one such case (toolchain == build product).

We also do not vendor Go modules. `go.sum` + `sum.golang.org` (Go's transparency log) + `go mod verify` (run in CI) + `govulncheck` give the same supply-chain guarantees as a committed `vendor/` directory without the ~10 MB repo bloat or the update friction (every dep bump = `go mod vendor` re-run + manual diff review). Vendor was the standard pre-Go-modules; the modern toolchain made it redundant for projects that build from a Go-team-operated proxy + checksum DB.

### 4.2 Build steps

1. **Compile our Go binary** in a Go builder image (Alpine-based, pinned by digest):

   ```
   CGO_ENABLED=0 go build -trimpath \
     -ldflags "-s -w -buildid= -X main.version=$VERSION -X main.commit=$COMMIT" \
     -o /out/wsl-vpnfix ./cmd/wsl-vpnfix
   ```

   `-trimpath` strips build paths, `-buildid=` zeros the nondeterministic Go build ID, no CGO so the binary is fully static.

2. **Fetch and verify upstream binaries:**

   ```
   curl -fSL <release>/sha256sums           → /tmp/sha256sums
   curl -fSL <release>/gvforwarder          → /tmp/gvforwarder
   curl -fSL <release>/gvproxy-windowsgui.exe → /tmp/gvproxy-windowsgui.exe
   sha256sum -c (filtered to our two artifacts) /tmp/sha256sums
   compare each line to build/upstream-pins.yaml; abort on any mismatch
   ```

3. **Assemble rootfs** with the pinned Alpine base. Install only `iproute2` and `nftables`. Remove apk caches before packing.

4. **Pack** as `wsl-vpnfix-<version>.tar.gz` ready for `wsl --import`.

5. **Generate metadata:**
   - `SHA256SUMS` for the tarball and `upstream-pins.yaml`.

### 4.3 Reproducibility

The build is reproducible if a clean rebuild from the same git tag, same Go toolchain version, same pinned Alpine digest, and same upstream pins yields a `wsl-vpnfix-<version>.tar.gz` with an identical SHA-256. Reproducibility is an internal quality property of `build/pack.sh`, not a separate CI attestation.

Bit-exact tarball reproducibility requires:

- Sorted file order in tar.
- Frozen `mtime` (`SOURCE_DATE_EPOCH = git commit time`).
- Frozen owner / mode (set explicitly, not from filesystem).
- Deterministic gzip (`gzip -n`).

### 4.4 CI / CD

GitHub Actions (free for public repos, standard runners on `ubuntu-24.04`). Rules:

- Every action pinned by commit SHA, not tag.
- Per-job `permissions:` blocks; default `read-all`; write only where strictly needed.
- `release.yml` upload job uses `permissions: { contents: write }` only; no OIDC token needed.
- Forked-PR workflows do not have access to release secrets (default GitHub behavior; not overridden).
- Renovate (`renovate.json` at repo root) with three customManager-driven streams: Go modules, Alpine + Go apk lockstep, `gvisor-tap-vsock` release. Weekly schedule, no auto-merge.

Workflows:

```
.github/workflows/
├── ci.yml       # PR: gofmt -l ., go vet ./... + go vet -tags=integration ./..., go mod verify, govulncheck ./..., unit + integration tests, build verify
└── release.yml  # tag-triggered (^vN.N.N$): runs build/pack.sh, uploads tarball + SHA256SUMS + upstream-pins.yaml to GH Release
```

`go mod verify` runs before any compilation and aborts the job if any cached module's content doesn't match the hash in `go.sum`. `govulncheck ./...` runs against the resolved module graph; currently non-blocking pending alpine apk shipping Go 1.25.10 (tracked in `TODO.md` Backlog). Together these substitute for vendoring (see 4.1 pinning policy).

### 4.5 Release artifacts

```
wsl-vpnfix-<version>.tar.gz   ← the importable WSL distro
SHA256SUMS                     ← sha256 of the tarball and upstream-pins.yaml
upstream-pins.yaml             ← what gvisor-tap-vsock version we shipped
```

### 4.6 Versioning

SemVer applies. The project currently sits on the 0.x line; no API or behavior stability commitment yet. There is no v1.0 audit gate. Phase A shipped v0.1.0; Phase C closes with v0.2.0.

Bump policy:

- **patch:** bug fix, security fix in our code, upstream pin bump.
- **minor:** new env var, new opt-in feature, Alpine major bump, documentation milestone (e.g. audit doc landing).
- **major:** breaking config change, dropping `wsl --import` compatibility, anything that changes user-observable behavior.

### 4.7 User update workflow

```
wsl --unregister wsl-vpnfix
sha256sum -c SHA256SUMS
wsl --import wsl-vpnfix $env:USERPROFILE\wsl-vpnfix wsl-vpnfix-1.2.3.tar.gz
```

A PowerShell helper (`scripts/install-wslvpnfix.ps1`) wraps the SHA-256 verify + import in one shot. The equivalent PowerShell check uses `Get-FileHash`. Verify is non-optional in the docs.

---

## 5. Repo layout

```
wsl-vpnfix/
├── cmd/
│   └── wsl-vpnfix/main.go              ← tiny main, delegates to internal/
├── internal/
│   ├── config/                          ← env parsing + strict validators
│   ├── netfilter/                       ← nft rules via netlink-typed lib
│   ├── netlink/                         ← tap creation, addr, route
│   ├── process/                         ← child mgmt, signal handling, reap
│   ├── wsl/                              ← WSL2 NAT gateway IP detector (resolv.conf)
│   └── healthcheck/                     ← optional connectivity probes
├── build/
│   ├── upstream-pins.yaml               ← gvisor-tap-vsock tag + sha256s
│   ├── Dockerfile.builder               ← reproducible Go build stage
│   ├── Dockerfile.rootfs                ← Alpine pinned by digest, final rootfs
│   ├── rootfs/                          ← wsl.conf, wsl-vpnfix.service, etc.
│   └── pack.sh                          ← deterministic tar assembly
├── .github/
│   └── workflows/
│       ├── ci.yml                       ← lint + test + build verify on PR
│       └── release.yml                  ← tag-triggered, uploads tarball + SHA256SUMS + upstream-pins.yaml
├── docs/
│   ├── SECURITY-AUDIT.md                ← findings + status, lives long-term
│   ├── THREAT-MODEL.md                  ← derived from #3, frozen reference
│   └── superpowers/specs/               ← design docs (this file)
├── scripts/
│   └── install-wslvpnfix.ps1            ← SHA-256 verify + wsl --import helper
├── CLAUDE.md
├── CONTRIBUTING.md
├── LICENSE                               ← BSD-2-Clause
├── README.md                             ← nanocontext-style: logo, badges, problem,
│                                          flow diagram, install matrix, anti-features
├── go.mod
└── go.sum
```

Layout notes:

- `internal/` keeps non-`main` packages un-importable by downstream consumers; surface to audit is exactly what `cmd/wsl-vpnfix` calls.
- `build/` co-locates the Go-builder image and the final rootfs image because they're sister pipelines.
- No `pkg/` (we expose nothing as a library).
- No `vendor/` (Go modules + sums + reproducible toolchain pin is enough).

---

## 6. Decisions

| Decision | Choice |
|---|---|
| Use case | Win 11 + WSL 2 + corporate VPN where mirrored mode does not suffice |
| Base image | Alpine, pinned by digest |
| Distribution model | Distro-only (sibling appliance distro, `wsl --import`) |
| Runtime | Go orchestrator + pinned upstream `gvforwarder` + pinned upstream `gvproxy-windowsgui.exe` |
| Architectures | amd64 only; ARM deferred |
| Network rules | nftables via netlink-typed Go library |
| IP plan / subnet | Compile-time fixed at `192.168.127.0/24` (gvproxy v0.8.8 hardcodes parts of it; runtime override would only pretend to work) |
| CI | GitHub Actions (free for public repos) |
| Signing | None; `SHA256SUMS` over GitHub TLS + tag immutability matches the audience's actual trust model |
| Dep inventory | None as a separate artifact; `go.sum` + `apk info -L` inside the rootfs cover the dep inventory — a separate file is duplicated state |
| Reproducibility | `-trimpath`, `-buildid=`, frozen mtime, deterministic gzip; internal build quality property, not a CI attestation |
| License | BSD-2-Clause |
| Default distro user | `root` (appliance, no human login) |
| PowerShell helper | `install-wslvpnfix.ps1` (fully lowercase per user request) |
| Microsoft Store distro registration | Deferred to v2 |
| Claude Code plugin | No; this is a system appliance, not a Claude tool |
| Logo / branding | Deferred (do not gate v1 on art) |
| Versioning | SemVer; currently 0.x line, no v1.0 audit gate; Phase C closes with v0.2.0 |

---

## 7. Out of scope for v1.0

- ARM64 Windows support.
- macOS, Windows 10, native Linux.
- Standalone install path (drop binaries into the user's primary distro).
- Microsoft Store distro registration.
- Claude Code plugin packaging.
- A custom logo.
- Replacing `gvproxy-windowsgui.exe` with our own user-space net stack.
- Importing `gvisor-tap-vsock` as a Go library to fold `gvforwarder` into our binary. Stay binary-pinned for now to avoid coupling to upstream's internal Go API.
- DoS resistance against a malicious sibling distro inside the same WSL VM.

---

## 8. Open items

All original open items are resolved during Phase A (netlink library and nftables library choices, `wsl-vpnfixctl` folded into the same binary as flags then redistributed to TODO Backlog in Phase C), Phase B (Go toolchain pin, Alpine digest pin, `gvisor-tap-vsock` release tag pin, PID 1 vs systemd resolved as `[boot] command=` under WSL `/init`), or Phase C (default-route persistence redistributed to TODO Backlog, fault-injection integration tests dropped). `TODO.md` is the canonical home for any future open work; this section is intentionally short to avoid duplicating state.

---

## 9. After this spec

1. User reviews this file. Changes round-trip through this doc until approved.
2. `superpowers:writing-plans` skill turns the approved spec into a step-by-step implementation plan with review checkpoints.
3. Implementation proceeds against the plan, not against the brainstorm transcript.
