<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# TODO — wsl-vpnfix

Open work only, ordered by temporal urgency: **Now** = active sprint or next-action-ready, **Later** = scheduled / has known shape but not started, **Backlog** = ideas not yet committed. When an item is done, delete the line. Don't duplicate decisions or rationale already captured in git log, plan/spec, or memory — link to those instead.

## Now

- [ ] **Plan Phase B (rootfs assembly + release pipeline).** Brainstorming pass first via `superpowers:brainstorming`; resolve open decisions: cosign keyless OIDC issuer, SLSA attestation level, GitHub Actions runners, init mechanism inside the appliance distro (PID 1 vs systemd) + hardening directives, Renovate config shape. Then write the plan via `superpowers:writing-plans` at `docs/superpowers/plans/<YYYY-MM-DD>-wsl-vpnfix-phase-b-rootfs-and-release.md`. Reference `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` section 4 for canonical inputs / build / repro / signing requirements; section 4.1's pinning policy table constrains every Phase B decision. Phase B Task 1 must be the production tarball + end-to-end smoke gate (`build/Dockerfile.rootfs` + `build/upstream-pins.yaml` + `build/pack.sh` → `out/wsl-vpnfix-X.Y.tar.gz`, then `wsl --import` + verify on a real WSL 2 host with corporate VPN connected) — that single task subsumes the deferred Phase A Task 14 and gives a real e2e gate before any pipeline work.

## Later

- [ ] **LICENSE file (BSD-2-Clause).** Workspace `CLAUDE.md` "Attribution & License" sets BSD-2-Clause as the default; the file is a 22-line standard form (year + name + boilerplate). Not blocking the Phase B plan, but blocks Phase B implementation because the appliance rootfs ships LICENSE inside.
- [ ] **Push to GitHub (private first).** No remote configured yet. Required before Phase B's cosign keyless OIDC signing flow works (cosign verifies against the GitHub Actions OIDC identity at `repo:zeroznet/wsl-vpnfix:ref:refs/tags/v…`). Switch from private → public when README + LICENSE land in Phase C.
- [ ] **Phase B follow-on tasks.** GitHub Actions CI/release/repro workflows, syft SBOM, hardened systemd unit (or PID-1 init wrapper), Renovate config that PRs `build/upstream-pins.yaml` and `go.mod` bumps separately. These are scoped inside the Phase B plan once written; listed here only so they don't slip out of view.

## Backlog

- [ ] **Phase C (public surface + audit).** README in nanocontext style, `install-wslvpnfix.ps1` Windows-side `wsl --import` helper, `docs/SECURITY-AUDIT.md` first pass (back-port findings from Phase A plan corrections C-1..C-8 plus anything Phase B reveals), `docs/THREAT-MODEL.md` derived from spec section 3, tag `v1.0.0`.
- [ ] **Periodic upstream check: `gvisor-tap-vsock` release.** Last verified upstream-latest 2026-05-09 (`v0.8.8`). When upstream cuts a new release: re-verify our orchestrator's spawn args still match (see `~/.claude/projects/-home-zero-dev-wsl-vpnfix/memory/project_gvisor_tap_vsock_v088.md` for the contract), bump pin in `build/upstream-pins.yaml` (when it exists), update memory note's "last re-verified" timestamp.
- [ ] **Periodic refresh: dev container base + Go toolchain.** When Alpine ships a new patch revision: bump `FROM alpine@sha256:…` digest in `dev/Containerfile`, bump `go=X.Y.Z-r0` apk pin in lockstep with `go.mod` `go` directive. Per workspace `CLAUDE.md` Behavioral Guideline #12 (Default to Latest Stable).
- [ ] **Phase A Task 14 manual smoke test.** Superseded by Phase B Task 1 (production tarball + smoke). Listed only as a pointer in case anyone wonders why Task 14 is marked "deferred" in the Phase A plan.
