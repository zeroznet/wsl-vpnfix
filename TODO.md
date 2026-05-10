<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# TODO — wsl-vpnfix

Open work only, ordered by temporal urgency: **Now** = active sprint or next-action-ready, **Later** = scheduled / has known shape but not started, **Backlog** = ideas not yet committed. When an item is done, delete the line. Don't duplicate decisions or rationale already captured in git log, plan/spec, or memory — link to those instead.

## Now

- [ ] **B4 work-PC validation: VPN-bypass smoke on the corp-VPN host.** Home-PC bridge validation green (`docs/smoke-2026-05-10.md`); the VPN-blackhole property is unproven until a run on the work PC with the misbehaving VPN connected. Procedure identical to the home-PC run (`docs/superpowers/plans/2026-05-09-wsl-vpnfix-phase-b-rootfs-and-release.md` Task B4). Tarball to import is whatever `build/pack.sh 0.1.0` produces from current `main`. On green: write `docs/smoke-2026-05-XX-workpc-vpn.md` and close B4 fully.
- [ ] **B5–B8: push to GitHub, ci.yml, release.yml, renovate.json.** Independent of B4's VPN run. Plan steps in `docs/superpowers/plans/2026-05-09-wsl-vpnfix-phase-b-rootfs-and-release.md` Tasks B5/B6/B7/B8. Continue under `superpowers:subagent-driven-development` next session.

## Later

- [ ] **Phase C (public surface + audit).** README in nanocontext style, `install-wslvpnfix.ps1` Windows-side `wsl --import` helper, `docs/SECURITY-AUDIT.md` first pass (back-port findings from Phase A plan corrections C-1..C-8 plus the four B3 follow-on commits documented in `docs/smoke-2026-05-10.md`), `docs/THREAT-MODEL.md` derived from spec section 3, tag `v1.0.0`. Phase C audit doc must explicitly reverse spec section 3.5's example finding F-007 — the current MASQUERADE rule is unscoped on purpose, see commit `0893652` and the smoke notes.

## Backlog

- [ ] **Periodic upstream check: `gvisor-tap-vsock` release.** Last verified upstream-latest 2026-05-09 (`v0.8.8`). v0.8.8 has a regression in `cmd/gvproxy/config.go:125` (the `-listen-stdio` CLI flag is parsed but never wired into `config.Interfaces.Stdio`); we work around it via the `-config` YAML emitted by `cmd/wsl-vpnfix/stage_exe.go`. When upstream cuts a new release: re-verify both the orchestrator spawn contract and the workaround-still-needed status (see `~/.claude/projects/-home-zero-dev-wsl-vpnfix/memory/project_gvisor_tap_vsock_v088.md`), bump pin in `build/upstream-pins.yaml`, drop the YAML workaround if the regression is fixed upstream, update the memory note.
- [ ] **Periodic refresh: dev container base + Go toolchain.** When Alpine ships a new patch revision: bump `FROM alpine@sha256:…` digest in `dev/Containerfile` and `build/Dockerfile.rootfs` (lockstep), bump `go=X.Y.Z-r0` apk pin in lockstep with `go.mod` `go` directive. Per workspace `CLAUDE.md` Behavioral Guideline #12.
- [ ] **Switch GitHub repo private → public.** Once Phase C lands README + LICENSE.
- [ ] **Master spec rebase (post-Phase-B cleanup).** `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` section 4 has drift versus what shipped. Phase B addendum at `docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md` is the authoritative override; once Phase B closes, fold the addendum's amendments into the master spec and delete or archive the addendum. Section 3.5's F-007 example also needs the reversal noted in the same pass.
