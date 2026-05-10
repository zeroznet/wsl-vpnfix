<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6) -->

# wsl-vpnfix — Threat Model

**Status:** Living reference, last reviewed 2026-05-10.

A standalone reference for the adversaries, trust boundaries, and audit scope of `wsl-vpnfix`. Lifted from `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` section 3 on 2026-05-10 to give the security audit catalogue (`docs/SECURITY-AUDIT.md`) a stable target to reference.

## 1. Adversaries

| # | Adversary | What they can do | Stance |
|---|---|---|---|
| 1 | Compromised process inside the user's dev distro (npm worm, malicious binary) | Same WSL VM, same kernel netns. Can send packets through our tap (by design — that is the feature). Cannot modify our nft rules without `CAP_NET_ADMIN` in the wsl-vpnfix netns. | In scope. Audit `/proc` exposure, fd leaks, config-file tampering. |
| 2 | Low-priv user on the Windows host | Plant DLLs near `wsl-gvproxy.exe`, attach to gvproxy process, write to interop paths. | Partially in scope. Audit interop launch — absolute paths, no PATH lookup, no DLL search-path attack. |
| 3 | Supply chain attacker | Compromises gvisor-tap-vsock release, our Go build, our Alpine base, our GitHub Actions secrets. | In scope, primary concern. Pin sourcing, checksum verification, reproducible builds, signed releases. |
| 4 | Network attacker past the VPN | Already inside the corporate net. Sees only TLS-encrypted traffic from us. | Out of scope (they have what they came for). |
| 5 | Linux kernel exploit / Windows OS exploit | Game over. | Out of scope. |
| 6 | Malicious user's own VPN client | Could see / modify everything we send to the host network. This is the production deployment shape: `wsl-vpnfix` runs alongside Cisco AnyConnect by design, validated in `docs/smoke-2026-05-10-workpc-vpn.md`. Not hypothetical. | Out of scope (user opted in by running it). |

## 2. Trust boundaries

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
                │     ├── SHA256SUMS over release artifacts │   release
                │     ├── reproducible build               │   pipeline
                │     └── verified upstream pins           │
                ▼
            end user
```

## 3. Assets

| Asset | Worst-case use |
|---|---|
| Our orchestrator binary integrity | Run arbitrary code as root inside wsl-vpnfix at boot. |
| Pinned upstream binary integrity | Same, plus traffic steering control. |
| The tap device + nft config | Redirect / sniff all WSL 2 traffic. |
| WSL host interop path (`/mnt/c/...`) | Drop fake `wsl-gvproxy.exe`, attach a debugger. |
| GitHub Actions secrets / signing identity | Ship malicious releases under our brand. |

## 4. In-scope audit checklist

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
- `wsl.conf`: interop enabled (we need it), `appendWindowsPath=false`, `automount.options="metadata,umask=22,fmask=11"`, `default=root`, orchestrator launched at boot via `[boot] command=/sbin/wsl-vpnfix` as a child of WSL's own `/init`.
- Orchestrator owns its own privilege model (root for the appliance lifetime, no drop), reaps zombies on `SIGCHLD`, forwards `SIGINT` / `SIGTERM` / `SIGHUP` into the existing ordered teardown stack.
- File modes: orchestrator and pinned binaries `0755 root:root`; checksums file `0644 root:root`.
- No setuid binaries beyond what Alpine ships and we actually need (audit `find / -perm -4000`).

**Build & release pipeline**

- Reproducible Go build: pinned toolchain in `go.mod` `toolchain` directive; CI matrix uses the same; `-trimpath -ldflags "-s -w -buildid="`; `CGO_ENABLED=0`.
- Pinned `containers/gvisor-tap-vsock` release tag. SHA-256s read from upstream's `sha256sums` artifact, which is itself verified against the release tag in our pipeline before any unpack.
- Alpine base image pinned by digest, not tag. Bumps via PR with one-line justification.
- Release artifacts: tarball, `SHA256SUMS`, `upstream-pins.yaml`.
- GitHub Actions: every action pinned by commit SHA, per-job `permissions:` blocks default `read-all`.

**WSL interop path**

- `wsl-gvproxy.exe` is invoked from a fixed absolute path inside our distro rootfs (copied at boot to where interop can reach), not from `/mnt/c/...`. The Windows side never reads from `%PATH%`.
- Stdio pipe is created in the orchestrator process. Pipe fds are passed only to the intended child and closed in the parent post-spawn.

## 5. Findings format

Findings are catalogued in `docs/SECURITY-AUDIT.md` as one-line entries grouped by area:

```
F-NNN <title> — <risk one-liner>; status: <commit-sha | tracked-in-TODO | fixed-in-v0.1.0>
```

Severity is implicit in the area grouping (`orchestrator`, `netfilter`, `supply-chain`, `wsl-interop`, `build`, `known-gaps`) rather than a separate field. The 7-field per-finding template in the original master spec was performative ceremony for findings that are mostly "small bug, fixed in commit X"; the short catalogue gives attention proportional to risk and survives the realistic maintenance cadence.

## 6. Re-audit cadence

Re-audit triggers:

- Any PR touching `internal/netfilter`, `internal/process`, `internal/wsl`, or the build pipeline.
- Any pinned-upstream major version bump (`gvisor-tap-vsock`, Alpine base, Go toolchain).
- Any Alpine base image bump beyond patch.

External eyes: the threat model and audit doc are public artifacts open to community comment. The audit doc is the trust artifact; the project does not say "trust us".

## 7. Out of scope

- Internals of `gvisor-tap-vsock` (we pin and verify, we do not read line-by-line).
- Linux kernel CVEs.
- Windows OS / Hyper-V.
- The user's VPN client.
- DoS resistance against a malicious sibling distro flooding the tap.
