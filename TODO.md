<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# TODO — wsl-vpnfix

Open work only. Decisions, lessons, and history live in git log + plan/spec; do not duplicate them here. When an item is done, delete the line. When something becomes load-bearing, move it into `CLAUDE.md` or the relevant plan/spec, not here.

## Phase B (production rootfs + release pipeline)

- [ ] Brainstorming pass on Phase B scope (decisions still open: cosign keyless flow + OIDC issuer, SLSA level, GitHub Actions runners, init mechanism inside the appliance distro — PID 1 vs systemd, hardening directives). Use `superpowers:brainstorming` skill before plan-writing.
- [ ] Write Phase B plan at `docs/superpowers/plans/<YYYY-MM-DD>-wsl-vpnfix-phase-b-rootfs-and-release.md`. Reference `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` section 4 for canonical inputs/build/repro/signing requirements; section 4.1 has the pinning policy that constrains every Phase B decision.
- [ ] Phase B Task 1 = production tarball (`build/Dockerfile.rootfs`, `build/upstream-pins.yaml`, `build/pack.sh` → `out/wsl-vpnfix-X.Y.tar.gz`) + end-to-end smoke gate via `wsl --import` on a real WSL 2 host with corporate VPN connected. This subsumes the deferred Phase A Task 14.
- [ ] Phase B follow-on tasks: GitHub Actions CI/release/repro workflows, cosign signing, syft SBOM, hardened systemd unit (or PID-1 init), Renovate config (`upstream-pins.yaml` + `go.mod` bump PRs).

## Phase C (public surface + audit)

- [ ] LICENSE file (BSD-2-Clause per project CLAUDE.md "Attribution & License" section). Tiny gap — could land standalone before Phase C if convenient.
- [ ] README in nanocontext style.
- [ ] `install-wslvpnfix.ps1` (Windows-side `wsl --import` helper).
- [ ] `docs/SECURITY-AUDIT.md` first pass — back-port findings from Phase A plan corrections C-1..C-8 plus anything Phase B reveals.
- [ ] `docs/THREAT-MODEL.md` derived from spec section 3.
- [ ] Tag `v1.0.0`.

## Off-cycle / housekeeping

- [ ] Push to GitHub (private first; public when README + LICENSE land). No remote configured yet. Required before cosign keyless OIDC works in Phase B.
- [ ] Periodic check: is `gvisor-tap-vsock` still on `v0.8.8`? Last verified 2026-05-09. Bump pin in `build/upstream-pins.yaml` (when it exists) + re-verify `internal/process` spawn args against new release.
- [ ] Periodic check: bump dev container `FROM alpine@sha256:...` digest in `dev/Containerfile` when Alpine ships a new patch revision; bump `go=X.Y.Z-r0` in lockstep with `go.mod` `go` directive.

## Blocked / waiting

- [ ] Phase A end-to-end gate (real WSL 2 host with corporate VPN). Not "blocked" technically — superseded by Phase B Task 1, which tests the production tarball flow instead of the synthetic dev flow.
