<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# TODO — wsl-vpnfix

Open work only, ordered by temporal urgency: **Now** = active sprint or next-action-ready, **Later** = scheduled / has known shape but not started, **Backlog** = ideas not yet committed. When an item is done, delete the line. Don't duplicate decisions or rationale already captured in git log, plan/spec, or memory — link to those instead.

## Now

## Later

- [ ] **Confirm Dependabot Updates runs under `sha_pinning_required`.** The repo requires SHA-pinned actions since 2026-11-20 (ci, vulnwatch and release pass). Unverified whether GitHub's dynamic "Dependabot Updates" workflow is exempt: check its next run under Actions; if it fails, turn the setting off or report it upstream.

- [ ] **Run `/ica` after some bake time.** End-to-end architectural review pass once the Phase A→C codebase has settled and a few real users have exercised it. Expect candidates in: orchestrator lifecycle (process supervision, signal flow, partial-init recovery), netfilter/netlink boundary (rule-set construction is one big function), config validation surface (env loader vs. Config struct duplication), test seam (no fault-injection harness was a deliberate Phase C drop, may want to revisit). Diagnose-only first; act on one item at a time per `/ica` discipline.

- [ ] **Keep `vulnwatch.yml` alive through quiet periods.** GitHub disables scheduled workflows in public repos after 60 days without repository activity (the 2026-07-30 → 2026-10-04 gap was 66 days), and a disabled schedule sends no failure mail. A bot-side keepalive is out: re-enabling via the API moves failure notifications to whoever re-enabled (the Actions bot), and the popular keepalive action's repo was taken down by GitHub for ToS. Candidate: an external weekly trigger running as Robert (`gh workflow enable vulnwatch.yml` + `gh workflow run vulnwatch.yml`). Until decided, merging a Renovate PR at least every ~7 weeks keeps it alive.

## Backlog

- [ ] **Optional: strict govulncheck PR gate via go.dev toolchain.** `ci.yml`'s govulncheck stays `continue-on-error: true` because alpine's apk go trails upstream Go security releases; alerting is covered by `vulnwatch.yml` (weekly strict scan of the shipped toolchain + Go EOL check). A blocking PR gate would need the builder + dev toolchain pivoted from apk go to the official go.dev tarball pinned by version + sha256 — separate design/PR if ever wanted.
- [ ] **Periodic upstream check: `gvisor-tap-vsock` release.** Last verified upstream-latest 2026-11-18 (`v0.9.0`, pinned). When upstream cuts a new release: follow the bump procedure in the `build/upstream-pins.yaml` header (spawn contract re-check, tag + commit + gvforwarder sha256, patch rebase or removal), bump both `GVTV_TAG` ARG defaults in `build/Dockerfile.rootfs`, and let CI's pins step confirm coherence.
- [ ] **Periodic refresh: dev container base + Go toolchain.** When Alpine ships a new patch revision: bump `FROM alpine@sha256:…` digest in `dev/Containerfile` and `build/Dockerfile.rootfs` (lockstep), bump `go=X.Y.Z-r0` apk pin in lockstep with `go.mod` `go` directive. Renovate's alpine + go PR moves only the apk pin, so bump the `go` directive in that same PR (9affd88 left it behind until #55). When a new Alpine minor ships, move the whole line (digests, apk go, and the `renovate.json` fence: `currentValueTemplate` + `alpine_3_NN/go`) — the old line's community repo freezes its apk go, which is how v0.2.3 ended up on an EOL Go 1.25. `vulnwatch.yml` fails once the shipped Go minor goes EOL. Per workspace `CLAUDE.md` Behavioral Guideline #12.
- [ ] **Drop `build/patches/gvproxy-hires-clock.patch` once upstream ships an equivalent fix.** The patch gives gvproxy's gVisor stack a QueryPerformanceCounter clock on Windows (root cause in `docs/SECURITY-AUDIT.md` F-019). Upstream fixed it on main in 3e179c63 (2026-11-05, issue #701; our duplicate #785), not yet in a release (latest v0.9.0). Their comment notes Go 1.28 gives the Windows monotonic clock 100 ns resolution, which makes both patches unnecessary once gvproxy builds with it. When a release carries the fix: delete the patch and the `gvproxy-builder` stage, pin the upstream `gvproxy-windowsgui.exe` by SHA-256 again, close F-019.
