<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Fable 5) -->

# wsl-vpnfix Toolchain Refresh (Alpine 3.23.5 + Go 1.25.10) — Design

**Date:** 2026-07-27
**Status:** Approved
**Author:** Robert Bopko, with Claude Fable 5 assistance

Unbreaks the build. Alpine's v3.23 repositories dropped the `go=1.25.9-r0` package when 1.25.10-r0 superseded it (verified against the live APKINDEX 2026-07-26), so `apk add go=1.25.9-r0` in `build/Dockerfile.rootfs` and `dev/Containerfile` now fails: the next release tag and any fresh dev-image build are broken. CI does not catch this because it provisions Go via `actions/setup-go`, never by building either Containerfile. This PR bumps the Alpine digest and Go pins in lockstep and replaces the dead "re-enable strict govulncheck" gate with an honest, permanent non-blocking posture.

---

## 1. Scope

One PR, pins + docs only. No runtime code, no dependency-graph changes beyond the `go.mod` toolchain directive.

Explicitly out of scope (each is its own follow-up PR): gvisor-tap-vsock v0.8.8 → v0.8.9 bump, `renovate.json` fixes and Renovate app installation, Alpine 3.24 / Go 1.26 line jump, `build/pack.sh` findings from the 2026-07-26 review sweep, all runtime-code findings.

---

## 2. Decision resolved

| # | Decision | Choice | Rationale |
|---|---|---|---|
| TR-D-1 | govulncheck gate strategy | Non-blocking permanently; track the strict-gate option in TODO Backlog | The original gate ("flip strict when alpine apk ships go 1.25.10") is structurally dead: a govulncheck run with go 1.25.10 (2026-07-26, local toolchain identical to the new apk pin) still reports 3 reachable stdlib CVEs — `GO-2026-5856` (crypto/tls ECH, fixed 1.25.12), `GO-2026-5039` (net/textproto, fixed 1.25.11), `GO-2026-5037` (crypto/x509, fixed 1.25.11). Alpine packages Go patch releases weeks after upstream security fixes (upstream is at 1.25.12 while apk carries 1.25.10), so a blocking gate against the apk toolchain would be red whenever upstream lands a fix — most of the time. The only clean path to a strict gate is building with the official go.dev toolchain pinned by version + sha256 instead of apk go; that is an architectural change to the reproducible build and gets its own design if ever pursued. |

---

## 3. Changes

### 3.1 Pin bumps (mechanical, one lockstep unit)

| File | Change |
|---|---|
| `dev/Containerfile:11` | `FROM alpine@sha256:4d889c14…` → `FROM alpine@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40` (3.23.4 → 3.23.5) |
| `build/Dockerfile.rootfs:12,46,68` | same digest bump, all three stages |
| `dev/Containerfile:14`, `build/Dockerfile.rootfs:19` | `go=1.25.9-r0` → `go=1.25.10-r0` |
| `go.mod:3` | `go 1.25.9` → `go 1.25.10` |

### 3.2 govulncheck posture (per TR-D-1)

`.github/workflows/ci.yml` — the step keeps `continue-on-error: true`; the comment above it changes from "re-enable once apk catches up" to the real reason. Replacement text:

> govulncheck runs non-blocking by design. The appliance builds with alpine's apk go, and alpine packages Go patch releases weeks after upstream security fixes, so a blocking gate would fail whenever a fresh stdlib CVE lands upstream — a structural lag, not a fixable state. Review the output when bumping the toolchain; the strict-gate option (go.dev toolchain pivot) is tracked in TODO.md.

`TODO.md` — Backlog item "Re-enable strict `govulncheck` in CI" is replaced by:

> **Periodic: review govulncheck output on toolchain bumps; strict gate stays off.** `ci.yml` runs govulncheck with `continue-on-error: true` permanently: the appliance builds with alpine's apk go, which trails upstream Go security releases by weeks (verified 2026-07-26: apk go 1.25.10 still trips `GO-2026-5037`/`GO-2026-5039`/`GO-2026-5856`, fixed upstream in 1.25.11/1.25.12). If a strict gate is ever wanted, the clean path is pivoting the builder + dev toolchain from apk go to the official go.dev tarball pinned by version + sha256 — separate design/PR if pursued.

`docs/SECURITY-AUDIT.md` — F-016 rewritten:

> F-016 govulncheck non-blocking in CI — CI scans with the apk-pinned Go toolchain, and alpine packages Go patch releases weeks behind upstream security fixes, so the current apk go usually carries a few reachable stdlib CVEs already fixed upstream (2026-07-26 on go 1.25.10: `GO-2026-5037` crypto/x509, `GO-2026-5039` net/textproto, `GO-2026-5856` crypto/tls ECH — all healthcheck-path, low exposure for this threat model; the two original blockers `GO-2026-4971` and `GO-2026-4918` are closed by the 1.25.10 bump); CI step stays `continue-on-error: true`; status: accepted (structural apk lag), strict-gate option (go.dev toolchain pivot) tracked in TODO (Backlog).

### 3.3 Live-truth sweep (same commit)

`CLAUDE.md` updates: line 126 (`go 1.25.9` → `go 1.25.10`), line 144 (`Alpine 3.23.4 … Go 1.25.9` → `Alpine 3.23.5 … Go 1.25.10`), the repo-layout `ci.yml` description ("non-blocking pending alpine apk go 1.25.10" → "non-blocking by design, see TODO"), and the Phase C Status sentence "CI follow-on still tracked: re-enable strict govulncheck once alpine apk ships Go 1.25.10 …" (rewritten to the accepted-structural-lag posture). Frozen plans/specs under `docs/superpowers/` are not touched.

Closing sweep: `grep -rn "1\.25\.9\|3\.23\.4\|4d889c14"` over live files must return nothing outside `docs/superpowers/` and `go.sum`.

---

## 4. Verification

1. **Pins resolve:** `podman build` of `dev/Containerfile` and of the `builder` stage of `build/Dockerfile.rootfs` completes (proves the 3.23.5 digest pulls and `go=1.25.10-r0` installs). The working session lacks a systemd user runtime dir, so podman runs with a session-scoped storage config; if that workaround fails, this check falls back to CI plus a local build on the maintainer's machine, stated explicitly in the PR.
2. **Tests:** `./dev/run.sh 'go test ./...'` green in the rebuilt dev image (or CI unit/integration/race jobs if local podman is unavailable).
3. **CI:** all jobs green on the PR branch; `actions/setup-go` resolves `go 1.25.10` from `go.mod` (exists upstream). The govulncheck step is expected to list exactly the 3 CVEs named in TR-D-1 and stay non-blocking.
4. **Sweep clean:** the closing grep from section 3.3.

---

## 5. Git flow

Branch `chore/alpine-3.23.5-go-1.25.10`; single-line commit message; PR body per `.github/PULL_REQUEST_TEMPLATE.md` (Summary + Test plan); merge commit after CI on protected `main`. Push happens only after explicit per-request authorization.
