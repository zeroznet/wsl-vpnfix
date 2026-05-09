<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix Phase B — Design Addendum

**Date:** 2026-05-09
**Status:** Addendum to `2026-05-08-wsl-vpnfix-design.md`. Supersedes section 4 of the master spec where conflicting; everything else in the master spec stands.
**Author:** Robert Bopko, with Claude Opus 4.7 assistance

Resolves the five Phase B open decisions called out in `TODO.md` Now-bucket and freezes the task decomposition. Inputs to `superpowers:writing-plans` for the actual step-by-step Phase B plan.

---

## 1. Decisions resolved

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D-1 | Init mechanism inside the appliance distro | **No systemd in the rootfs. Orchestrator launched at boot via `wsl.conf` `[boot] command=/sbin/wsl-vpnfix` as a child of WSL's own `/init`.** | Original decision was "Go orchestrator runs as PID 1." The "no systemd binaries" half holds — rootfs ships zero systemd / dbus / journald, dropping ~10 MB of CVE / audit surface vs a systemd-based appliance. The PID-1 half was reversed on 2026-05-09 after smoke testing on Windows 11 + WSL kernel 6.6.87.2: setting `[boot] systemd=true` (the only WSL mechanism to exec `/sbin/init` as PID 1) drops WSL interop into a half-broken state where gvproxy.exe's stdio bridge to gvforwarder goes one-way (TX only, RX zero). The systemd-mode integration WSL expects (sd_notify, systemctl, user sessions) we cannot satisfy without shipping systemd — the very thing this decision was meant to avoid. Upstream `sakai135/wsl-vpnkit` uses `[boot] command=` for the same reason. The PID-1 reaper code in `cmd/wsl-vpnfix/init.go` (B2) stays in the repo as defensive cover; `os.Getpid() != 1` so it no-ops at runtime. |
| D-2 | Release signing | **Drop cosign keyless entirely** | Upstream `sakai135/wsl-vpnkit` ships an unsigned tarball; we ship a tarball plus `SHA256SUMS`, which is one step stricter and matches the audience's actual trust model (GitHub TLS plus tag immutability). Strict-identity-regex ceremony added complexity disproportionate to threat for v1.0. |
| D-3 | Supply-chain release artifacts | **Tarball, `SHA256SUMS`, `upstream-pins.yaml`. No SBOM, no SLSA attestation, no `.sig` / `.pem`.** | SBOM (CycloneDX via syft) was the only marginal call. `go.sum` plus `apk info -L` inside the rootfs already give the dep inventory anyone who asks would actually use; a separate file is duplicated state (workspace CLAUDE.md guideline #10). SLSA depends on the cosign chain and falls with D-2. |
| D-4 | GitHub Actions runner image | **`ubuntu-24.04` (pinned LTS)** | Workspace CLAUDE.md guideline #12 (immutable identity). `ubuntu-latest` is a moving alias; the bump is GitHub's call, not ours. Pinning costs zero; bump is a one-line PR with explicit CI re-run. |
| D-5 | Automated dep-bump PRs | **Renovate, three separate streams, weekly schedule, no auto-merge** | Renovate `customManagers / regexManagers` parses the `tag` and `sha256:` lines in `build/upstream-pins.yaml`; Dependabot does not. Three streams (Go modules / Alpine digest plus Go apk in lockstep / `gvisor-tap-vsock`) because each has a different audit ritual: Go bumps run `go mod verify` plus `govulncheck` plus tests; Alpine plus Go apk bumps require a dev-container rebuild plus smoke; gvisor-tap-vsock bumps require re-verifying spawn contracts in `~/.claude/projects/-home-zero-dev-wsl-vpnfix/memory/project_gvisor_tap_vsock_v088.md`. Auto-merge disabled because supply-chain bumps are exactly the surface where supply-chain attacks materialize. |

---

## 2. Master-spec amendments

The decisions in #1 reshape `2026-05-08-wsl-vpnfix-design.md` section 4. The amendments below are authoritative for Phase B; the master spec remains the long-term reference and should be edited to fold these in once Phase B ships (a v1.0 cleanup task, not a Phase B blocker).

| Master-spec section | Phase B amendment |
|---|---|
| 2.6 Rootfs contents | Drop `/etc/systemd/system/wsl-vpnfix.service`. Use `wsl.conf` `[boot] command=/sbin/wsl-vpnfix` to autostart the orchestrator at distro boot as a child of WSL's own `/init` (NOT as PID 1). `systemd=true` was tried on 2026-05-09 and broke WSL interop's stdio bridge to gvproxy.exe due to systemd-style integration assumptions WSL makes that we cannot satisfy without the systemd binary surface. Upstream `sakai135/wsl-vpnkit` uses the same `command=` approach. The `/sbin/init` symlink to `/sbin/wsl-vpnfix` stays in the rootfs (cheap defensive cover) but is unused at runtime. The PID-1 reaper code in `cmd/wsl-vpnfix/init.go` (B2) does not fire because `os.Getpid() != 1`; it remains as defensive cover for a hypothetical future PID-1 rehome. Final rootfs: `/sbin/wsl-vpnfix`, `/sbin/wsl-gvforwarder`, `/etc/wsl-vpnfix/wsl-gvproxy.exe`, `/etc/wsl-vpnfix/checksums`, `/etc/wsl.conf` (with `command=/sbin/wsl-vpnfix`), `/sbin/init` symlink, `LICENSE`, plus busybox, Alpine baselayout, `nftables`, `iproute2`, `ca-certificates`. No `bash`, no `curl`, no `wget`, no compilers, no SSH. |
| 3.4 distro rootfs hardening | Drop the systemd service-unit hardening bullet. Replace with: orchestrator owns its own privilege model (root for the appliance lifetime, no drop), runs as PID 1, reaps zombies on `SIGCHLD`, forwards `SIGINT` / `SIGTERM` / `SIGHUP` into the existing ordered teardown stack. No setuid binaries beyond what Alpine's `busybox-suid` ships and we add nothing extra. |
| 4.2 Build steps | Remove the SBOM step (formerly 5b). Steps 1 through 5a stand. |
| 4.3 Reproducibility | Keep all build flags (`-trimpath`, `-buildid=`, `SOURCE_DATE_EPOCH`, `gzip -n`, sorted tar, fixed owner / mode). Drop the dedicated `reproducibility.yml` workflow; reproducibility becomes an internal quality property of the build script, not a release attestation. |
| 4.4 Signing | Section deleted. |
| 4.5 CI / CD | Workflows reduce to `ci.yml` and `release.yml`. Both run on `ubuntu-24.04`. `release.yml` does not need `id-token: write`. `ci.yml` runs `gofmt -l .`, `go vet ./...` and `go vet -tags=integration ./...`, `go mod verify`, `govulncheck ./...`, unit and integration tests, and a build verify. Renovate config (`renovate.json`) lives at the repo root with three customManager-driven streams. |
| 4.6 Release artifacts | Final list: `wsl-vpnfix-X.Y.Z.tar.gz`, `SHA256SUMS`, `upstream-pins.yaml`. Drop `.sig`, `.pem`, `.cdx.json`, `SHA256SUMS.sig`. |
| 4.8 User update workflow | Replace the `cosign verify-blob` block with `sha256sum -c SHA256SUMS` (or a PowerShell `Get-FileHash` equivalent in `install-wslvpnfix.ps1`, deferred to Phase C). |

Master-spec sections 1, 2 (excluding 2.6), 3 (excluding 3.4), 5, 6, 7, 8, 9 stand unchanged.

---

## 3. Task decomposition

Eight tasks, linear dependency order. The pattern mirrors the Phase A plan: one subsystem per task, the smoke gate consumes prior tasks, pipeline tasks come after the smoke gate proves the artifact.

| # | Task | Output | Depends on |
|---|---|---|---|
| B1 | LICENSE file (BSD-2-Clause, year 2026, holder Robert Bopko). | `LICENSE` at repo root. | — |
| B2 | PID-1 init implementation in the orchestrator: branch on `os.Getpid() == 1` at startup; install a `SIGCHLD` reaper goroutine using `wait4(-1, …, WNOHANG)`; mount `/proc` if absent; forward `SIGINT` / `SIGTERM` / `SIGHUP` into the teardown stack already wired in `cmd/wsl-vpnfix/main.go`. Unit tests covering the reaper using isolated child processes. No new flag — PID detection is the trigger so `go test` keeps running in non-init mode. | `cmd/wsl-vpnfix/init.go`, `cmd/wsl-vpnfix/init_test.go`, possibly `internal/process/reaper.go` if the reaper extracts cleanly. | B1. |
| B3 | Production rootfs: `build/Dockerfile.rootfs` (`FROM alpine@sha256:…` plus `apk add --no-cache nftables iproute2 ca-certificates` only); `build/upstream-pins.yaml` (gvisor-tap-vsock tag plus per-artifact sha256); `build/pack.sh` (POSIX shell, `set -eu`, deterministic tar via `--sort=name`, `--mtime=@$SOURCE_DATE_EPOCH`, fixed owner / mode; `gzip -n`); produces `out/wsl-vpnfix-0.1.0.tar.gz` reproducibly. | `build/` tree; `./build/pack.sh 0.1.0` runs from a clean checkout. | B2 (binary needs PID-1 support to be valid in rootfs). |
| B4 | **End-to-end smoke gate** on Robert's Win 11 host with the corp VPN connected: `wsl --import wsl-vpnfix C:\…\wsl-vpnfix out\wsl-vpnfix-0.1.0.tar.gz`; from a sibling distro verify ICMP, DNS, HTTPS, healthchecks; capture pre / post `ip route`, `nft list ruleset`, kmsg, orchestrator stdout. Notes recorded at `docs/smoke-2026-05-XX.md`. **Green B4 closes deferred Phase A Task 14 and gates B5 onward.** | smoke notes file; merge gate for B5+. | B3. |
| B5 | Push to GitHub as a private repo at `github.com/zeroznet/wsl-vpnfix`. Configure branch protection on `main`: require pull-request before merge, require status checks (`ci.yml`) to pass, no force-push, no direct pushes. Approval requirement deferred until a second contributor exists. | `origin` remote configured; protection rules in place. | B4 (do not push artifacts that have not passed smoke). |
| B6 | `.github/workflows/ci.yml` on `ubuntu-24.04`: `gofmt -l .`, `go vet ./...` plus `go vet -tags=integration ./...`, `go mod verify`, `govulncheck ./...`, unit tests (`go test ./...`), integration tests (`go test -tags=integration ./...` with `NET_ADMIN` plus `NET_RAW` plus `/dev/net/tun`), build verify. Every action pinned by commit SHA. Workflow-level `permissions: read-all`; no per-job overrides needed (CI never writes). | working PR-check pipeline. | B5. |
| B7 | `.github/workflows/release.yml` triggered by `on.push.tags: ['v*.*.*']` (glob), with a guard step inside the job that re-checks the tag against the strict regex `^v[0-9]+\.[0-9]+\.[0-9]+$` and fails fast on mismatch. Runs `build/pack.sh $TAG`, computes `SHA256SUMS` over the tarball plus `upstream-pins.yaml`, uploads all three to a GitHub Release. Workflow-level `permissions: read-all`; the upload job overrides with `permissions: { contents: write }`. No `id-token: write`. | working tag-to-release pipeline. | B6. |
| B8 | `renovate.json` at repo root with three streams: Go modules via the built-in `gomod` manager; Alpine `FROM alpine@sha256:…` plus the matching `go=X.Y.Z-r0` apk pin in `dev/Containerfile` and `build/Dockerfile.rootfs` linked together; gvisor-tap-vsock `tag:` plus `sha256:` lines in `upstream-pins.yaml` via a `regexManager`. Schedule `before 6am on Monday`. `automerge: false` everywhere. PR labels per stream (`deps:go`, `deps:alpine`, `deps:gvisor-tap-vsock`) so the review queue is filterable. | working bot pipeline. | B7. |

---

## 4. Out of scope for Phase B

Named so they do not leak in:

- **Default-route persistence to disk for crash-safe recovery** (master spec section 8 open item). Phase B's smoke gate does not exercise mid-init crash recovery. Tracked in `TODO.md` Backlog as a Phase C follow-on; not a B4 blocker.
- **Fault-injection integration tests** for partial-init teardown (master spec section 8 open item). Same reasoning; Phase B ships without them; Phase C adds them before v1.0.
- **`wsl-vpnfixctl` debug subcommand** (master spec section 8 open item). User-facing diagnostics belong with the Phase C public-surface work.
- **README, `install-wslvpnfix.ps1`, `docs/SECURITY-AUDIT.md`, `docs/THREAT-MODEL.md`** — explicitly Phase C per `CLAUDE.md` status section.
- **Pre-release / `rc` tag support in `release.yml`** — the workflow accepts only `v[0-9]+\.[0-9]+\.[0-9]+`. If pre-release becomes useful, relax in a follow-on PR.
- **Tarball signing of any kind**. Reverses D-2.

---

## 5. After this addendum

1. User reviews this file.
2. `superpowers:writing-plans` skill turns the approved addendum (combined with master spec sections 1–3 and the unaffected parts of 4–9) into the Phase B step-by-step implementation plan at `docs/superpowers/plans/2026-05-09-wsl-vpnfix-phase-b-rootfs-and-release.md`.
3. Implementation proceeds against that plan.
