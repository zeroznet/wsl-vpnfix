<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Fable 5) -->

# Toolchain Refresh (Alpine 3.23.5 + Go 1.25.10) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unbreak the build (alpine dropped `go=1.25.9-r0` from the v3.23 repo) by bumping the Alpine digest and Go pins in lockstep, and replace the dead "re-enable strict govulncheck" gate with the permanent non-blocking posture decided in TR-D-1.

**Architecture:** No code changes. Six pin lines across three files, four documentation surfaces (ci.yml comment, TODO.md, SECURITY-AUDIT F-016, CLAUDE.md), one closing sweep. Spec: `docs/superpowers/specs/2026-07-27-toolchain-refresh-design.md`.

**Tech Stack:** podman (session-scoped storage), apk pins, GitHub Actions CI.

## Global Constraints

- Workspace guideline #11: pins and every doc that describes them change in the SAME commit — Tasks 1, 3, 4 only stage; Task 5 makes the single implementation commit.
- Commit messages: single line, no `Co-Authored-By`, lowercase start.
- Push and PR creation only after explicit user authorization (Task 6 gates on it).
- Frozen history under `docs/superpowers/plans/` and dated specs is never edited (this plan file and the 2026-07-27 spec are the current, non-frozen additions).
- New Alpine digest (3.23.5): `sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40`. New apk pin: `go=1.25.10-r0`. New go.mod directive: `go 1.25.10`.
- Working branch: `chore/alpine-3.23.5-go-1.25.10` (already exists, carries the spec commit `165105b`).

---

### Task 1: Pin bumps (lockstep unit)

**Files:**
- Modify: `dev/Containerfile:11,14`
- Modify: `build/Dockerfile.rootfs:12,19,46,68`
- Modify: `go.mod:3`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: working tree with all six pin lines updated; Task 2 builds it, Task 5 commits it.

- [ ] **Step 1: Bump the digest in all four FROM lines**

In `dev/Containerfile` (1 occurrence) and `build/Dockerfile.rootfs` (3 occurrences: stages builder, fetcher, final), replace every:

```
alpine@sha256:4d889c14e7d5a73929ab00be2ef8ff22437e7cbc545931e52554a7b00e123d8b
```

with:

```
alpine@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40
```

- [ ] **Step 2: Bump both apk go pins**

In `dev/Containerfile:14` and `build/Dockerfile.rootfs:19`, replace `go=1.25.9-r0` with `go=1.25.10-r0`. Keep the trailing ` \` continuation exactly as is.

- [ ] **Step 3: Bump the go.mod directive**

In `go.mod:3`, replace `go 1.25.9` with `go 1.25.10`. Do not touch `go.sum` (toolchain directive changes do not alter the module graph).

- [ ] **Step 4: Verify edit completeness**

Run: `grep -rn "1\.25\.9\|4d889c14" dev/Containerfile build/Dockerfile.rootfs go.mod`
Expected: no output.

Run: `grep -c "fd791d74" dev/Containerfile build/Dockerfile.rootfs`
Expected: `dev/Containerfile:1` and `build/Dockerfile.rootfs:3`.

- [ ] **Step 5: Stage (no commit yet — Global Constraints)**

```bash
git add dev/Containerfile build/Dockerfile.rootfs go.mod
```

---

### Task 2: Prove the new pins resolve (podman builds)

**Files:**
- No edits. Builds the Task 1 working tree.

**Interfaces:**
- Consumes: Task 1's edited working tree.
- Produces: evidence for the PR Test plan (build logs); dev image for the test run in Task 5.

The session shell has no systemd user runtime dir, so the user's default podman storage is unusable here (`database run root … mismatch`). Use fully session-scoped storage — fresh `--root` AND `--runroot` — so the user's podman state is never touched.

- [ ] **Step 1: Define session-scoped podman wrapper**

```bash
SP=/home/zero/tmp/claude-1000/-home-zero-dev-wsl-vpnfix/d3331237-18e6-4956-b38d-a60ed2ee7129/scratchpad
mkdir -p "$SP/podman/root" "$SP/podman/runroot"
PODMAN="podman --root $SP/podman/root --runroot $SP/podman/runroot --cgroup-manager=cgroupfs --events-backend=file"
```

- [ ] **Step 2: Build the dev image**

Run: `$PODMAN build -t wsl-vpnfix-dev-test -f dev/Containerfile dev/`
Expected: completes; the `apk add` layer installs `go-1.25.10-r0` (visible in the layer log). This proves both the 3.23.5 digest and the apk pin resolve.

- [ ] **Step 3: Build the rootfs builder stage**

Run: `$PODMAN build --target builder -f build/Dockerfile.rootfs .`
Expected: completes; proves the release-path Dockerfile pulls the new base, installs `go=1.25.10-r0`, and compiles `./cmd/wsl-vpnfix` with it (default ARG values suffice for the builder stage; GVTV args belong to the fetcher stage, not built here).

- [ ] **Step 4 (fallback, only if Steps 2-3 fail on podman infrastructure): document and defer**

If podman cannot run even with session-scoped storage (runtime errors unrelated to the Dockerfiles), record the exact error, state in the PR Test plan that local build verification was blocked by the sandbox and that (a) the live APKINDEX check (2026-07-26) proves `go=1.25.10-r0` is the served version, and (b) the maintainer should run `podman build -f dev/Containerfile dev/` locally once. Do NOT claim the build was verified.

---

### Task 3: govulncheck posture (ci.yml comment + TODO.md item)

**Files:**
- Modify: `.github/workflows/ci.yml:50-53`
- Modify: `TODO.md:15`

**Interfaces:**
- Consumes: nothing from prior tasks (text-only).
- Produces: staged doc changes for the Task 5 commit.

- [ ] **Step 1: Replace the ci.yml gate comment**

Replace exactly these four lines (currently `ci.yml:50-53`, directly above the `- name: govulncheck` step; `continue-on-error: true` on the step is KEPT):

```yaml
      # govulncheck runs but does not gate the build while the alpine apk go
      # package (build/Dockerfile.rootfs's go=X.Y.Z-rN pin) lags upstream Go.
      # Re-enable strict mode once apk ships the Go release that closes the
      # currently flagged stdlib CVEs.
```

with:

```yaml
      # govulncheck runs non-blocking by design. The appliance builds with
      # alpine's apk go, and alpine packages Go patch releases weeks after
      # upstream security fixes, so a blocking gate would fail whenever a
      # fresh stdlib CVE lands upstream — structural lag, not a fixable
      # state. Review the output when bumping the toolchain; the strict-gate
      # option (go.dev toolchain pivot) is tracked in TODO.md.
```

- [ ] **Step 2: Replace the TODO.md Backlog item**

Replace the entire line `TODO.md:15` (the `- [ ] **Re-enable strict `govulncheck` in CI.** …` bullet) with:

```markdown
- [ ] **Periodic: review govulncheck output on toolchain bumps; strict gate stays off.** `ci.yml` runs govulncheck with `continue-on-error: true` permanently: the appliance builds with alpine's apk go, which trails upstream Go security releases by weeks (verified 2026-07-26: apk go 1.25.10 still trips `GO-2026-5037`/`GO-2026-5039`/`GO-2026-5856`, fixed upstream in 1.25.11/1.25.12). If a strict gate is ever wanted, the clean path is pivoting the builder + dev toolchain from apk go to the official go.dev tarball pinned by version + sha256 — separate design/PR if pursued.
```

- [ ] **Step 3: Stage**

```bash
git add .github/workflows/ci.yml TODO.md
```

---

### Task 4: F-016 + CLAUDE.md live truth

**Files:**
- Modify: `docs/SECURITY-AUDIT.md:41`
- Modify: `CLAUDE.md:111,126,144,147`

**Interfaces:**
- Consumes: nothing from prior tasks (text-only).
- Produces: staged doc changes for the Task 5 commit.

- [ ] **Step 1: Rewrite F-016**

Replace the entire line `docs/SECURITY-AUDIT.md:41` (the `- F-016 govulncheck non-blocking in CI — Go 1.25.9 carries 2 stdlib CVEs …` bullet) with:

```markdown
- F-016 govulncheck non-blocking in CI — CI scans with the apk-pinned Go toolchain, and alpine packages Go patch releases weeks behind upstream security fixes, so the current apk go usually carries a few reachable stdlib CVEs already fixed upstream (2026-07-26 on go 1.25.10: `GO-2026-5037` crypto/x509, `GO-2026-5039` net/textproto, `GO-2026-5856` crypto/tls ECH — all healthcheck-path, low exposure for this threat model; the two original blockers `GO-2026-4971` and `GO-2026-4918` are closed by the 1.25.10 bump); CI step stays `continue-on-error: true`; status: accepted (structural apk lag), strict-gate option (go.dev toolchain pivot) tracked in TODO (Backlog).
```

- [ ] **Step 2: Update CLAUDE.md line 111 (Phase C status paragraph, final sentence)**

Replace:

```
CI follow-on still tracked: re-enable strict `govulncheck` once alpine apk ships Go 1.25.10 (currently 2 stdlib CVEs flagged non-blocking — low exposure for this threat model).
```

with:

```
CI follow-on resolved 2026-07-27: `govulncheck` stays non-blocking by design — alpine's apk go structurally trails upstream Go security releases, so a strict gate would fail whenever upstream lands a fix (TR-D-1 in `docs/superpowers/specs/2026-07-27-toolchain-refresh-design.md`); the original 2 stdlib CVEs are closed by the go 1.25.10 bump.
```

- [ ] **Step 3: Update CLAUDE.md repo-layout lines**

Line 126: replace `go 1.25.9` with `go 1.25.10`.
Line 144: replace `Alpine 3.23.4 (digest-pinned) + Go 1.25.9 dev image` with `Alpine 3.23.5 (digest-pinned) + Go 1.25.10 dev image`.
Line 147: replace `govulncheck (non-blocking pending alpine apk go 1.25.10)` with `govulncheck (non-blocking by design — apk go trails upstream, see TODO.md)`.

- [ ] **Step 4: Stage**

```bash
git add docs/SECURITY-AUDIT.md CLAUDE.md
```

---

### Task 5: Closing sweep, tests, single implementation commit

**Files:**
- No new edits expected (sweep may surface stragglers — fix them in place).

**Interfaces:**
- Consumes: all staged changes from Tasks 1, 3, 4; dev image from Task 2.
- Produces: the single implementation commit on `chore/alpine-3.23.5-go-1.25.10`.

- [ ] **Step 1: Closing sweep (Global Constraints / guideline #11)**

Run: `grep -rn "1\.25\.9\|3\.23\.4\|4d889c14" --include="*.md" --include="*.yml" --include="*.yaml" --include="*.mod" --include="Containerfile" --include="*.rootfs" . | grep -v docs/superpowers | grep -v go.sum`
Expected: no output. Any hit outside frozen docs is a missed reference — fix and re-stage before committing.

- [ ] **Step 2: Unit tests in the Task 2 dev image**

Run: `$PODMAN run --rm -v "$PWD":/work -w /work wsl-vpnfix-dev-test go test ./...`
Expected: all packages `ok`. (Integration/race need caps and CGO — CI covers them.) If Task 2 fell back, skip and rely on CI; say so in the PR Test plan.

- [ ] **Step 3: Review the staged diff as one unit**

Run: `git diff --cached --stat && git diff --cached`
Expected: exactly 7 files — `dev/Containerfile`, `build/Dockerfile.rootfs`, `go.mod`, `.github/workflows/ci.yml`, `TODO.md`, `docs/SECURITY-AUDIT.md`, `CLAUDE.md` — and every hunk traces to the spec.

- [ ] **Step 4: Commit (single line)**

```bash
git commit -m "deps(alpine): bump base digest to 3.23.5 and go to 1.25.10 in lockstep (apk dropped 1.25.9-r0, release build was broken); govulncheck stays non-blocking by design per TR-D-1, ci.yml/TODO/F-016/CLAUDE.md updated to match"
```

- [ ] **Step 5: Clean up session podman storage**

```bash
rm -rf "$SP/podman"
```

---

### Task 6: Push, PR, CI, merge (GATED on user authorization)

**Files:**
- No edits.

**Interfaces:**
- Consumes: branch `chore/alpine-3.23.5-go-1.25.10` with spec commit + implementation commit.
- Produces: merged PR on `main`.

- [ ] **Step 1: Freshness check**

Run: `git fetch origin && git log --oneline origin/main -1 && git log origin/main..HEAD --oneline`
Expected: origin/main unchanged from `6b19d37` (else rebase the branch first); exactly 2 commits ahead (spec + implementation).

- [ ] **Step 2: ASK THE USER for push authorization**

Do not push without an explicit yes for this specific push. Quote the branch name and the 2 commits.

- [ ] **Step 3: Push and open PR (after yes)**

```bash
git push -u origin chore/alpine-3.23.5-go-1.25.10
gh pr create --title "toolchain refresh: alpine 3.23.5 + go 1.25.10 lockstep, govulncheck non-blocking by design" --body "$(cat <<'EOF'
## Summary
- alpine v3.23 dropped `go=1.25.9-r0` when 1.25.10-r0 superseded it, so `apk add` in both Containerfiles fails: next release tag and any fresh dev-image build were broken (CI never builds these images, so it stayed green). Bumps the base digest to 3.23.5 and go to 1.25.10 across all pins in lockstep (4 FROM lines, 2 apk pins, go.mod directive).
- Replaces the dead strict-govulncheck gate: verified that go 1.25.10 still trips 3 stdlib CVEs fixed upstream in 1.25.11/1.25.12, and apk structurally trails upstream Go security releases, so the gate is non-blocking by design now (TR-D-1 in the spec). ci.yml comment, TODO.md, SECURITY-AUDIT F-016, and CLAUDE.md updated to match.

## Test plan
- [ ] `podman build -f dev/Containerfile dev/` pulls 3.23.5 and installs go-1.25.10-r0
- [ ] `podman build --target builder -f build/Dockerfile.rootfs .` compiles the orchestrator with the new toolchain
- [ ] `go test ./...` green in the rebuilt dev image
- [ ] CI green (setup-go resolves go 1.25.10 from go.mod); govulncheck step lists GO-2026-5037/5039/5856 and stays non-blocking
EOF
)"
```

Adjust the Test plan checkboxes to reflect what actually ran (Task 2 fallback ⇒ say so).

- [ ] **Step 4: Wait for CI, then merge**

Run: `gh pr checks --watch`
Expected: `ci` SUCCESS. Then merge with a merge commit (squash/rebase are disabled repo-wide):

```bash
gh pr merge --merge
```

If blocked on unresolved review threads (Codex), resolve them first per the memory note `project_pr_merge_dance_with_codex.md`.

- [ ] **Step 5: Post-merge verification**

Run: `git checkout main && git pull && git log --oneline -3 && git branch -d chore/alpine-3.23.5-go-1.25.10`
Expected: merge commit on main; local branch deleted (remote branch auto-deletes via `delete_branch_on_merge`).
