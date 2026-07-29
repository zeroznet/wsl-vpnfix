<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Fable 5) -->

# wsl-vpnfix gvisor-tap-vsock v0.8.9 Bump — Design

**Date:** 2026-07-29
**Status:** Approved
**Author:** Robert Bopko, with Claude Fable 5 assistance

Bumps the pinned `containers/gvisor-tap-vsock` release from v0.8.8 to v0.8.9 (released 2026-05-11). v0.8.9 fixes exactly the `--listen-stdio` config-propagation regression our staged-YAML workaround exists for ("fix(gvproxy): propagate --listen-stdio flag into config"), plus a udp_proxy tight-loop bug (read returns 0 with nil err — live relevance: DNS transits gvproxy over UDP) and a `--log-file` fix. Pin-only bump: the workaround stays.

---

## 1. Decisions resolved

| # | Decision | Choice | Rationale |
|---|---|---|---|
| GV-D-1 | YAML workaround | Retained; removal is a separate post-smoke PR | The `-config` path works unchanged on v0.8.9, so the bump carries zero behavior change and the PR is verifiable without a WSL host (CI fetcher guard checks tag/sha against real artifacts). Dropping the workaround now would bundle unverifiable code changes with the bump and blur fault isolation. Removal PR must re-add `ssh-port=-1` to the stdio URL — leaving `-config` mode re-enables gvproxy default mode and its 127.0.0.1:2222 SSH listener (audit F-011). |
| GV-D-2 | Smoke flow | Merge → tag `v0.2.2` → installer upgrade on work PC under Cisco AnyConnect | Smokes the artifact end users actually receive (release tarball via `scripts/install-wslvpnfix.ps1`), same as the v0.2.1 validation. Tag protection means a failed smoke is fixed forward (v0.2.3), never by re-pointing. The upgrade event doubles as the diagnostics opportunity for the tracked Start-ScheduledTask post-upgrade issue (capture `Get-ScheduledTaskInfo` per the TODO item). |

Spawn-contract re-verification (TODO playbook step 1) already done 2026-07-29 against the upstream clone: `cmd/vm`, `pkg/net/stdio`, and `pkg/transport/dial_linux.go` are byte-identical between v0.8.8 and v0.8.9; the only transport changes are unixgram/vfkit-related (not our path). gvproxy-side changes are the stdio wiring fix, `--log-file` ordering, ec2-metadata flag description, and the udp_proxy fix.

---

## 2. Changes

| File | Change |
|---|---|
| `build/upstream-pins.yaml` | `tag: v0.8.8` → `v0.8.9`; gvforwarder sha256 → `a62731c3e07e6d98b26043d236f4d03c9e2d464d75f1f3ec3670e5b2825eb6a6`; gvproxy-windowsgui.exe sha256 → `53fa495ea057f43751ab8da57fd72fef62052ac6c4eecb2d5161c40157c7fb79` (from the official v0.8.9 `sha256sums` release asset; re-fetched at implementation time to confirm) |
| `build/Dockerfile.rootfs:48` | `ARG GVTV_TAG=v0.8.8` → `ARG GVTV_TAG=v0.8.9` (fallback default kept in lockstep with the pins file) |
| `cmd/wsl-vpnfix/stage_exe.go` | Comment-only: note that upstream fixed the wiring in v0.8.9 and the workaround is retained until the post-smoke removal PR |
| `docs/SECURITY-AUDIT.md` (F-017) | Status updated: upstream fix confirmed in v0.8.9 (config.go wires `args.stdioSocket` → `config.Interfaces.Stdio`, verified in source); workaround deliberately retained; removal tracked in TODO with the F-011 `ssh-port=-1` caveat |
| `TODO.md` | Periodic-check item: last-verified date → 2026-07-29 / v0.8.9 pinned, next-release wording kept; new Backlog item: drop the YAML workaround after the v0.8.9 smoke passes (re-add `ssh-port=-1`, F-011; update memory note `project_gvisor_tap_vsock_v088.md`) |

Out of scope: workaround removal code, memory-note rewrite (post-smoke), CLAUDE.md release paragraph (written when `v0.2.2` is tagged).

---

## 3. Verification

1. Re-fetch `https://github.com/containers/gvisor-tap-vsock/releases/download/v0.8.9/sha256sums` and confirm both values match the spec before editing.
2. PR CI: the "Verify upstream pins (fetcher stage)" step (added in PR #32) downloads the real v0.8.9 artifacts and runs `sha256sum -c` against the new pins — the decisive automated check this PR was designed to be caught by.
3. Post-merge (GV-D-2): tag `v0.2.2` (separate push authorization) → release pipeline → installer upgrade on work PC → smoke under AnyConnect (`wsl -d <distro> -- curl -sI https://1.1.1.1`, any HTTP response = bridge works). Capture Start-ScheduledTask diagnostics during the upgrade per the TODO installer item.

## 4. Git flow

Branch `chore/gvtv-v0.8.9`; spec + plan + one implementation commit; PR per template; merge commit after CI; push and tag each gated on explicit authorization.
