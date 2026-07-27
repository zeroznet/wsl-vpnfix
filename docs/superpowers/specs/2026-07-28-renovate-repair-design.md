<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Fable 5) -->

# wsl-vpnfix Renovate Repair — Design

**Date:** 2026-07-28
**Status:** Approved
**Author:** Robert Bopko, with Claude Fable 5 assistance

Makes the just-activated Renovate bot safe to obey. The Mend Renovate GitHub App was never installed during Phase B — the config sat inert for 11 weeks (zero PRs; discovered when the stale `go=1.25.9-r0` pin broke the release build, fixed in PR #30). The app is now installed (2026-07-28, Scan and Alert mode), which arms two latent config bugs found in the 2026-07-26 review sweep: a grouped "lockstep" PR that would produce an unbuildable image, and gvisor-tap-vsock tag bumps that merge green but break the next release. This PR fixes both before the bot's first scheduled run (Monday 2026-08-03).

---

## 1. Scope

Config + CI guard + doc truth. Four files: `renovate.json`, `build/upstream-pins.yaml` (header comment only), `.github/workflows/ci.yml` (one new step), `CLAUDE.md`.

Out of scope: gvisor-tap-vsock v0.8.9 bump (next PR — the new CI guard must land first so the bump PR is properly checked), `build/pack.sh` header fixes (later cleanup PR), `TODO.md` (no item affected).

---

## 2. Decisions resolved

| # | Decision | Choice | Rationale |
|---|---|---|---|
| RR-D-1 | Guard against tag-only gvtv bumps | Always-run fetcher-stage build in CI | Renovate's regex customManager bumps only the `tag:` line of `build/upstream-pins.yaml`; the sha256 values cannot be derived by regex and stay manual. Without a guard such a PR is CI-green (CI never builds the Dockerfile) and the breakage surfaces at the next release tag. An always-on `podman build --target fetcher` step (~30-60 s, two ~MB-scale downloads from GitHub releases) turns the mismatch red in the PR itself and continuously proves the pinned artifacts are still downloadable. Conditional-on-diff execution was rejected: saves little, adds fetch-depth/diff plumbing, and loses the availability check. |
| RR-D-2 | Dependency dashboard | Enabled permanently | After 11 weeks of a silently dead bot, a standing heartbeat is worth one pinned GitHub issue. The dashboard appears on the bot's first run after merge (not gated by the Monday schedule), which doubles as the verification that the app is alive and the config parses. |
| RR-D-3 | Alpine digest manager fence | Track the `3.23` tag, not `latest` | `currentValueTemplate: "latest"` would propose a 3.24 digest (alpine:latest = 3.24.1) while the repology manager pins `alpine_3_23/go` — the grouped PR merges green (CI does not build the Containerfiles) and produces an image where `apk add go=1.25.10-r0` fails. Fencing the digest manager to `3.23` keeps both managers on one line; a major-line move (3.24 / Go 1.26) becomes a deliberate manual PR that bumps the fence, the repology package name, the apk pin, and go.mod together. |

---

## 3. Changes

### 3.1 `renovate.json`

- `extends`: remove `":disableDependencyDashboard"` (keeps `config:recommended`, `":semanticCommits"`).
- Alpine digest customManager: `"currentValueTemplate": "latest"` → `"currentValueTemplate": "3.23"`.

### 3.2 `build/upstream-pins.yaml` (header comment only, pins untouched)

Replace the header paragraph and bump procedure with:

> Pinned dependencies for the wsl-vpnfix release tarball. Renovate's customManager (renovate.json) bumps ONLY the `tag:` line below when upstream cuts a release; the sha256 values are filled in manually per the procedure here. CI's "Verify upstream pins" step builds the fetcher stage on every run, so a tag/sha mismatch fails the PR instead of the release.
>
> Bump procedure:
>   1. Verify the new release does not break the spawn contract documented in `~/.claude/projects/-home-zero-dev-wsl-vpnfix/memory/project_gvisor_tap_vsock_v088.md`
>   2. Fetch the new sha256sums: `curl -fsSL https://github.com/containers/gvisor-tap-vsock/releases/download/<TAG>/sha256sums`
>   3. Replace the tag and the two sha256 values below.
>   4. Re-run `build/pack.sh`; smoke test on a real WSL host.

This drops the false "Renovate … parses the `tag:` and `sha256:` lines", the stale "manual until the bot lands", and the B3/B4 plan-speak. The attribution header line stays as authored.

### 3.3 `.github/workflows/ci.yml` — new step "Verify upstream pins (fetcher stage)"

Placed after "Build verify", before "Race detector". Exact step:

```yaml
      # Mirrors build/pack.sh's awk parsing of build/upstream-pins.yaml.
      # Runs on every CI pass so a tag-only Renovate bump (sha256 lines are
      # maintained manually) turns the PR red here instead of breaking the
      # release-tag build later, and continuously proves the pinned
      # artifacts are still downloadable.
      - name: Verify upstream pins (fetcher stage)
        run: |
          set -eu
          PINS=build/upstream-pins.yaml
          GVTV_TAG=$(awk '/^  tag:/ { print $2; exit }' "$PINS")
          GVF_SHA=$(awk '/^    gvforwarder:/ { f=1; next } f && /sha256:/ { print $2; exit }' "$PINS")
          GVP_SHA=$(awk '/^    gvproxy-windowsgui\.exe:/ { f=1; next } f && /sha256:/ { print $2; exit }' "$PINS")
          test -n "$GVTV_TAG" && test -n "$GVF_SHA" && test -n "$GVP_SHA"
          podman build \
            --target fetcher \
            --file build/Dockerfile.rootfs \
            --build-arg "GVTV_TAG=$GVTV_TAG" \
            --build-arg "GVTV_GVFORWARDER_SHA256=$GVF_SHA" \
            --build-arg "GVTV_GVPROXY_EXE_SHA256=$GVP_SHA" \
            .
```

podman is preinstalled on `ubuntu-24.04` runners and matches `build/pack.sh`'s runtime preference.

### 3.4 `CLAUDE.md` live truth

- Repo-layout `renovate.json` line: "3 streams: gomod, alpine+go-apk lockstep, gvisor-tap-vsock release" (already stale — the github-actions stream landed in PR #17) → "4 streams: gomod, alpine-3.23 digest + go-apk lockstep, gvisor-tap-vsock tag (sha256 manual, CI-guarded), github-actions".
- Repo-layout `ci.yml` line: insert "upstream-pins verify" between "build verify" and "race" (matches step order).
- Status section: new short paragraph "**Renovate activated** as of 2026-07-28" recording: app was never installed during Phase B, config inert 11 weeks, discovered via the stale-pin build breakage (PR #30); app installed in Scan and Alert mode; dashboard enabled; digest manager fenced to 3.23 (RR-D-3); CI fetcher-stage guard added (RR-D-1).

---

## 4. Verification

1. `renovate.json` parses as JSON (`python3 -m json.tool`).
2. The PR's own CI run exercises the new step against the current v0.8.8 pins — must pass (also proves the awk extraction works on the real file).
3. Local negative test if network permits: same fetcher build with one corrupted sha256 build-arg — expect `sha256sum -c` failure (proves the guard actually bites). Not repeatable in CI; evidence goes in the PR Test plan.
4. Post-merge, within hours: the "Dependency Dashboard" issue appears in the repo — proof the app is alive and the config parses. If instead the bot files a config-error issue, fix forward.

---

## 5. Git flow

Branch `chore/renovate-repair`; spec + plan + one implementation commit; PR per template; merge commit after CI; push only after explicit per-request authorization.
