<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix — Design

**Date:** 2026-05-08
**Status:** Draft, pending user review
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

```
/sbin/wsl-vpnfix              ← our Go binary (orchestrator)
/sbin/wsl-gvforwarder         ← pinned upstream Linux binary
/etc/wsl-vpnfix/wsl-gvproxy.exe
                              ← pinned upstream Windows binary, shipped here;
                                copied at boot to a path WSL interop can launch
/etc/wsl.conf                 ← interop on, automount opts hardened, systemd on
/etc/systemd/system/wsl-vpnfix.service
/etc/wsl-vpnfix/checksums     ← SHA-256 of bundled binaries, verified at boot
```

(Exact paths may shift slightly during plan; this is the intent.)

---

## 3. Threat model & audit scope

### 3.1 Adversaries

| # | Adversary | What they can do | Stance |
|---|---|---|---|
| 1 | Compromised process inside the user's dev distro (npm worm, malicious binary) | Same WSL VM, same kernel netns. Can send packets through our tap (by design — that is the feature). Cannot modify our nft rules without `CAP_NET_ADMIN` in the wsl-vpnfix netns. | In scope. Audit `/proc` exposure, fd leaks, config-file tampering. |
| 2 | Low-priv user on the Windows host | Plant DLLs near `wsl-gvproxy.exe`, attach to gvproxy process, write to interop paths. | Partially in scope. Audit interop launch — absolute paths, no PATH lookup, no DLL search-path attack. |
| 3 | Supply chain attacker | Compromises gvisor-tap-vsock release, our Go build, our Alpine base, our GitHub Actions secrets. | In scope, primary concern. Pin sourcing, checksum verification, reproducible builds, signed releases. |
| 4 | Network attacker past the VPN | Already inside the corporate net. Sees only TLS-encrypted traffic from us. | Out of scope (they have what they came for). |
| 5 | Linux kernel exploit / Windows OS exploit | Game over. | Out of scope. |
| 6 | Malicious user's own VPN client | Could see / modify everything we send to the host network. | Out of scope (user opted in by running it). |

### 3.2 Trust boundaries

```
Internet ── (VPN handles) ── Windows host
                                 │
                                 │ (b) WSL interop boundary             ← AUDIT
                                 │     ├── how we launch wsl-gvproxy.exe
                                 │     ├── stdio pipe ownership
                                 │     └── absolute paths, no PATH lookup
                                 │
                              WSL 2 VM
                                 │
                ┌────────────────┴─────────────────────────┐
                │ (c) shared kernel netns                  │ ← AUDIT
                │     ├── nft rules from us                │   wsl-vpnfix
                │     ├── /proc visibility from siblings   │   vs sibling
                │     └── tap device mode / permissions    │   distros
                │                                          │
        wsl-vpnfix distro ─── sibling ──── user's primary distro
                │
                │ (d) build → install boundary             ← AUDIT
                │     ├── signed releases (cosign)         │   release
                │     ├── reproducible build               │   pipeline
                │     └── verified upstream pins           │
                ▼
            end user
```

### 3.3 Assets

| Asset | Worst-case use |
|---|---|
| Our orchestrator binary integrity | Run arbitrary code as root inside wsl-vpnfix at boot. |
| Pinned upstream binary integrity | Same, plus traffic steering control. |
| The tap device + nft config | Redirect / sniff all WSL 2 traffic. |
| WSL host interop path (`/mnt/c/...`) | Drop fake `wsl-gvproxy.exe`, attach a debugger. |
| GitHub Actions secrets / signing identity | Ship malicious releases under our brand. |

### 3.4 In-scope audit checklist

**Our Go code (`wsl-vpnfix`)**

- All env / config inputs validated against strict regex before use; no value flows unvalidated into syscalls or subprocess args.
- Subprocess invocation uses absolute paths, explicit empty `Env` (or copy-then-allowlist), no shell.
- Child fd ownership tracked; no fd leaks to children that do not need them.
- Signal handling is total: SIGINT / SIGTERM / SIGHUP all teardown cleanly; children reaped; no zombies.
- nftables rules built via the netlink-typed library, not by string-formatting argv.
- No `os/exec` use of the shell. No `bash -c`. No string concatenation into command lines.
- Logs never include resolved env values, fd numbers, or anything that helps a co-resident attacker fingerprint state.

**Distro rootfs**

- Alpine base image pinned by digest. Package list audited — only what runtime requires (no `bash`, no `curl`, no `wget`, no compilers, no SSH).
- `wsl.conf`: interop enabled (we need it), `appendWindowsPath=false`, `automount.options="metadata,umask=22,fmask=11"`, `default=root`, `systemd=true`.
- Service unit hardening: `ProtectSystem=strict`, `PrivateTmp=true`, `NoNewPrivileges=true`, narrow `CapabilityBoundingSet` (`CAP_NET_ADMIN` only), minimal `ReadWritePaths=`.
- File modes: orchestrator and pinned binaries `0755 root:root`; checksums file `0644 root:root`.
- No setuid binaries beyond what Alpine ships and we actually need (audit `find / -perm -4000`).

**Build & release pipeline**

- Reproducible Go build: pinned toolchain in `go.mod` `toolchain` directive; CI matrix uses the same; `-trimpath -ldflags "-s -w -buildid="`; `CGO_ENABLED=0`.
- Pinned `containers/gvisor-tap-vsock` release tag. SHA-256s read from upstream's `sha256sums` artifact, which is itself verified against the release tag in our pipeline before any unpack.
- Alpine base image pinned by digest, not tag. Bumps via PR with one-line justification.
- Release artifacts: tarball, `SHA256SUMS`, `SHA256SUMS.sig`, SBOM (`syft` → CycloneDX JSON).
- GitHub Actions: every action pinned by commit SHA, per-job `permissions:` blocks default `read-all`, `id-token: write` only on the release job.

**WSL interop path**

- `wsl-gvproxy.exe` is invoked from a fixed absolute path inside our distro rootfs (copied at boot to where interop can reach), not from `/mnt/c/...`. The Windows side never reads from `%PATH%`.
- Stdio pipe is created in the orchestrator process. Pipe fds are passed only to the intended child and closed in the parent post-spawn.

### 3.5 Findings format

Each finding lives in `docs/SECURITY-AUDIT.md`:

```
### F-007  nft MASQUERADE scoped too broadly
Severity: medium
Area:     orchestrator / netfilter
File:     internal/netfilter/rules.go:42
Repro:    bring up wsl-vpnfix, run `nft list ruleset` from a sibling distro,
          observe rule scope.
Risk:     packets from non-WSL sources (impossible today, but assumed in plan)
          would also be NAT'd, masking their origin.
Fix:      narrow with `oifname "wsltap" ip saddr 192.168.127.0/24`.
Status:   open / fixed in <commit>
```

### 3.6 Audit cadence

- **Initial audit:** before tagging `v1.0.0`. No public release before the audit doc lands.
- **Re-audit triggers:** any PR touching `internal/netfilter/`, `internal/process/`, `internal/wsl/`, or the build pipeline; any pinned-upstream major version bump; any Alpine base image bump.
- **External eyes:** the audit doc + threat model are public artifacts open to community comment. The doc is the trust artifact; we do not say "trust us".

### 3.7 Out of scope

- Internals of `gvisor-tap-vsock` (we pin and verify, we do not read line-by-line).
- Linux kernel CVEs.
- Windows OS / Hyper-V.
- The user's VPN client.
- DoS resistance against a malicious sibling distro flooding the tap.

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
   - `SHA256SUMS` for the tarball and every shipped binary.
   - SBOM via `syft packages dir:./rootfs -o cyclonedx-json` → `wsl-vpnfix-<version>.cdx.json`.
   - Build provenance via SLSA-style attestation.

### 4.3 Reproducibility

The build is reproducible if a clean rebuild from the same git tag, same Go toolchain version, same pinned Alpine digest, and same upstream pins yields a `wsl-vpnfix-<version>.tar.gz` with an identical SHA-256.

CI enforces this with a `reproducibility.yml` workflow that runs the full build twice on independent runners and fails the release on any diff.

Bit-exact tarball reproducibility requires:

- Sorted file order in tar.
- Frozen `mtime` (`SOURCE_DATE_EPOCH = git commit time`).
- Frozen owner / mode (set explicitly, not from filesystem).
- Deterministic gzip (`gzip -n`).

### 4.4 Signing

Keyless signing via Sigstore / cosign + GitHub OIDC:

```
cosign sign-blob --yes \
  --output-signature wsl-vpnfix-<version>.tar.gz.sig \
  --output-certificate wsl-vpnfix-<version>.tar.gz.pem \
  wsl-vpnfix-<version>.tar.gz
```

No long-lived signing keys. Signatures verify against the GitHub Actions OIDC identity (`repo:zeroznet/wsl-vpnfix:ref:refs/tags/v…`) using `cosign verify-blob`.

### 4.5 CI / CD

GitHub Actions (free for public repos, standard runners). Rules:

- Every action pinned by commit SHA, not tag.
- Per-job `permissions:` blocks; default `read-all`; write only where strictly needed.
- `id-token: write` only on the release job.
- Forked-PR workflows do not have access to release secrets (default GitHub behavior; not overridden).
- Renovate (or Dependabot) configured to PR `upstream-pins.yaml` bumps with the new sha256sums staged ready to verify.

Workflows:

```
.github/workflows/
├── ci.yml                 # PR: lint + test + build verify
├── release.yml            # tag-triggered: reproducible build, sign, SBOM, upload
└── reproducibility.yml    # tag + nightly: rebuild + diff
```

### 4.6 Release artifacts

```
wsl-vpnfix-<version>.tar.gz       ← the importable WSL distro
wsl-vpnfix-<version>.tar.gz.sig   ← cosign signature
wsl-vpnfix-<version>.tar.gz.pem   ← cosign certificate
wsl-vpnfix-<version>.cdx.json     ← CycloneDX SBOM
SHA256SUMS                         ← sha256 of every artifact above
SHA256SUMS.sig                     ← cosign signature of SHA256SUMS
upstream-pins.yaml                 ← what gvisor-tap-vsock version we shipped
```

### 4.7 Versioning

- SemVer from `v1.0.0`. `v0.x` while the audit doc is still being written; `v1.0.0` ships only after the initial audit lands.
- Bump policy:
  - **patch:** bug fix, security fix in our code, upstream pin bump.
  - **minor:** new env var, new opt-in feature, Alpine major bump.
  - **major:** breaking config change, dropping `wsl --import` compatibility, anything that changes user-observable behavior.

### 4.8 User update workflow

```
wsl --unregister wsl-vpnfix
cosign verify-blob \
  --certificate wsl-vpnfix-1.2.3.tar.gz.pem \
  --signature   wsl-vpnfix-1.2.3.tar.gz.sig \
  --certificate-identity-regexp "^https://github.com/zeroznet/wsl-vpnfix/" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  wsl-vpnfix-1.2.3.tar.gz
wsl --import wsl-vpnfix $env:USERPROFILE\wsl-vpnfix wsl-vpnfix-1.2.3.tar.gz
```

A PowerShell helper (`scripts/install-wslvpnfix.ps1`) wraps the verify + import in one shot. Verify is non-optional in the docs.

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
│       ├── release.yml                  ← tag-triggered, cosign-signed
│       └── reproducibility.yml          ← second-build diff job
├── docs/
│   ├── SECURITY-AUDIT.md                ← findings + status, lives long-term
│   ├── THREAT-MODEL.md                  ← derived from #3, frozen reference
│   └── superpowers/specs/               ← design docs (this file)
├── scripts/
│   └── install-wslvpnfix.ps1            ← cosign verify + wsl --import helper
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
| Architectures | amd64 only at v1.0; ARM deferred |
| Network rules | nftables via netlink-typed Go library |
| IP plan / subnet | Compile-time fixed at `192.168.127.0/24` (gvproxy v0.8.8 hardcodes parts of it; runtime override would only pretend to work) |
| CI | GitHub Actions (free for public repos) |
| Signing | Sigstore / cosign keyless via GitHub OIDC |
| SBOM | syft, CycloneDX JSON |
| Reproducibility | `-trimpath`, `-buildid=`, frozen mtime, deterministic gzip, second-build diff in CI |
| License | BSD-2-Clause |
| Default distro user | `root` (appliance, no human login) |
| PowerShell helper | `install-wslvpnfix.ps1` (fully lowercase per user request) |
| Microsoft Store distro registration | Deferred to v2 |
| Claude Code plugin | No; this is a system appliance, not a Claude tool |
| Logo / branding | Deferred (do not gate v1 on art) |
| Versioning | SemVer; v0.x during audit, v1.0.0 after initial audit lands |

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

## 8. Open items (resolve in plan or first PRs)

- Exact pinned Go toolchain version at v1.0 cut.
- Exact pinned Alpine digest at v1.0 cut.
- Exact pinned `gvisor-tap-vsock` release tag at v1.0 cut.
- Final choice between `vishvananda/netlink` and the newer `mdlayher/netlink` family for our netlink layer (decide during plan after a small spike).
- Final choice of nftables Go library (`google/nftables` vs hand-rolled netlink expressions).
- Whether the orchestrator runs as PID 1 directly or under systemd. Default position: under systemd because we want the unit hardening, but the trade-off (extra ~10 MB, more moving parts) gets a final look in the plan.
- Whether to ship a tiny `wsl-vpnfixctl` debug subcommand (`status`, `dump-config`, `verify-pins`) inside the same binary, or as a separate flag set on the same binary.
- Default-route capture is the only non-idempotent recovery step in init: we delete the existing WSL2 default and stash the original in RAM before installing our tap default. If the orchestrator dies between the delete and the install, restart cannot recover the original route — `CaptureAndDelDefaultRoutes` finds nothing to capture. Decide before v1.0 whether to persist the captured route(s) to disk (e.g. `/run/wsl-vpnfix/saved-routes.json`) for crash-safe recovery, or accept that a `systemctl restart wsl-vpnfix` mid-init may require a `wsl --shutdown` to recover networking in the primary distro.
- Partial-init teardown is verified only by Task 14's manual smoke test in Phase A. Phase decomposition in `cmd/wsl-vpnfix/main.go` makes per-phase failure paths visible, but no automated test forces a mid-init failure at each phase boundary and asserts kernel state cleanliness (tap gone, no nft table, original default routes restored). Before v1.0, add fault-injection integration tests — either by mocking the netlink/netfilter packages (requires interface indirection at the orchestrator boundary) or by forcing real failures from a privileged test harness (e.g. pre-create a conflicting tap to make `CreateTap` fail, then assert teardown reverses the prior phase's state).

---

## 9. After this spec

1. User reviews this file. Changes round-trip through this doc until approved.
2. `superpowers:writing-plans` skill turns the approved spec into a step-by-step implementation plan with review checkpoints.
3. Implementation proceeds against the plan, not against the brainstorm transcript.
