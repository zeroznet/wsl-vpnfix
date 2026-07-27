<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Fable 5) -->

# Renovate Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the just-activated Renovate bot safe to obey — fix the two latent config bugs (lockstep contradiction, tag-only gvtv bumps) before its first scheduled run on Monday 2026-08-03.

**Architecture:** Config edits (`renovate.json`), one honest header rewrite (`build/upstream-pins.yaml`), one always-on CI guard step (`.github/workflows/ci.yml` fetcher-stage build), CLAUDE.md live truth. Spec: `docs/superpowers/specs/2026-07-28-renovate-repair-design.md` (decisions RR-D-1..3).

**Tech Stack:** Renovate hosted app (Mend, installed 2026-07-28), GitHub Actions, podman.

## Global Constraints

- Guideline #11: all four files change in ONE implementation commit — Tasks 1-4 stage only; Task 5 commits.
- Commit messages single line, no Co-Authored-By, lowercase start.
- Push/PR/merge only after explicit user authorization (Task 6 gates).
- Pins in `build/upstream-pins.yaml` (tag v0.8.8 + both sha256) are NOT touched — header comment only.
- Working branch: `chore/renovate-repair` (exists, carries spec commit `212d083`).
- Session podman needs session-scoped storage: `PODMAN="podman --root $SP/podman/root --runroot $SP/podman/runroot --cgroup-manager=cgroupfs --events-backend=file"` with `SP=/home/zero/tmp/claude-1000/-home-zero-dev-wsl-vpnfix/d3331237-18e6-4956-b38d-a60ed2ee7129/scratchpad`. `podman build` networking works in this session; `podman run` networking does not.

---

### Task 1: renovate.json — dashboard on, digest fence to 3.23

**Files:**
- Modify: `renovate.json:6` (extends entry), `renovate.json:70` (currentValueTemplate)

**Interfaces:**
- Consumes: nothing.
- Produces: staged config for the Task 5 commit.

- [ ] **Step 1: Remove the dashboard disable**

In the `extends` array, delete the line `    ":disableDependencyDashboard"` AND the trailing comma on the preceding `":semanticCommits"` line, leaving:

```json
  "extends": [
    "config:recommended",
    ":semanticCommits"
  ],
```

- [ ] **Step 2: Fence the alpine digest manager**

In the first customManager (depNameTemplate "alpine"), replace `"currentValueTemplate": "latest",` with `"currentValueTemplate": "3.23",`.

- [ ] **Step 3: Verify JSON parses**

Run: `python3 -m json.tool renovate.json > /dev/null && echo JSON-OK`
Expected: `JSON-OK`.

- [ ] **Step 4: Stage**

```bash
git add renovate.json
```

---

### Task 2: upstream-pins.yaml — honest header

**Files:**
- Modify: `build/upstream-pins.yaml:1-13` (comment block only; `gvisor_tap_vsock:` block below stays byte-identical)

**Interfaces:**
- Consumes: nothing.
- Produces: staged header for the Task 5 commit.

- [ ] **Step 1: Replace the header comment block**

Replace exactly these lines (current 1-13):

```
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)
#
# Pinned dependencies for the wsl-vpnfix release tarball. Renovate's
# regexManager (configured in renovate.json) parses the `tag:` and
# `sha256:` lines below and opens PRs when the upstream cuts a new release.
#
# Bump procedure (manual until the bot lands):
#   1. Verify the new release does not break the spawn contract documented
#      in ~/.claude/projects/-home-zero-dev-wsl-vpnfix/memory/project_gvisor_tap_vsock_v088.md
#   2. Fetch the new sha256sums:
#        curl -fsSL https://github.com/containers/gvisor-tap-vsock/releases/download/<TAG>/sha256sums
#   3. Replace the tag and the two sha256 values below.
#   4. Re-run B3's pack.sh; B4's smoke test on a real WSL host.
```

with:

```
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)
#
# Pinned dependencies for the wsl-vpnfix release tarball. Renovate's
# customManager (renovate.json) bumps ONLY the `tag:` line below when
# upstream cuts a release; the sha256 values are filled in manually per
# the procedure here. CI's "Verify upstream pins" step builds the fetcher
# stage on every run, so a tag/sha mismatch fails the PR instead of the
# release.
#
# Bump procedure:
#   1. Verify the new release does not break the spawn contract documented
#      in ~/.claude/projects/-home-zero-dev-wsl-vpnfix/memory/project_gvisor_tap_vsock_v088.md
#   2. Fetch the new sha256sums:
#        curl -fsSL https://github.com/containers/gvisor-tap-vsock/releases/download/<TAG>/sha256sums
#   3. Replace the tag and the two sha256 values below.
#   4. Re-run build/pack.sh; smoke test on a real WSL host.
```

- [ ] **Step 2: Verify pins untouched**

Run: `grep -c "v0.8.8\|4b674c0f\|8803caf8" build/upstream-pins.yaml`
Expected: `3` (tag + both sha256 lines unchanged).

- [ ] **Step 3: Stage**

```bash
git add build/upstream-pins.yaml
```

---

### Task 3: ci.yml — "Verify upstream pins (fetcher stage)" step

**Files:**
- Modify: `.github/workflows/ci.yml` (insert between the "Build verify" step and the "Race detector (CGO required)" step)

**Interfaces:**
- Consumes: nothing.
- Produces: staged workflow for the Task 5 commit; the step name "Verify upstream pins (fetcher stage)" is referenced by CLAUDE.md in Task 4 as "upstream-pins verify".

- [ ] **Step 1: Insert the step**

Between "Build verify" and "Race detector (CGO required)", insert exactly:

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

- [ ] **Step 2: YAML sanity check**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" 2>/dev/null && echo YAML-OK || echo "pyyaml unavailable — rely on CI parse"`
Expected: `YAML-OK` (or the fallback note; a parse error would fail the PR's CI immediately either way).

- [ ] **Step 3: Stage**

```bash
git add .github/workflows/ci.yml
```

---

### Task 4: Local guard proof (positive + negative)

**Files:**
- No edits. Exercises the exact commands the new CI step runs.

**Interfaces:**
- Consumes: Task 2's working tree (pins file), `build/Dockerfile.rootfs` fetcher stage.
- Produces: evidence for the PR Test plan.

- [ ] **Step 1: Positive — current pins must pass**

```bash
SP=/home/zero/tmp/claude-1000/-home-zero-dev-wsl-vpnfix/d3331237-18e6-4956-b38d-a60ed2ee7129/scratchpad
mkdir -p "$SP/podman/root" "$SP/podman/runroot"
PODMAN="podman --root $SP/podman/root --runroot $SP/podman/runroot --cgroup-manager=cgroupfs --events-backend=file"
PINS=build/upstream-pins.yaml
GVTV_TAG=$(awk '/^  tag:/ { print $2; exit }' "$PINS")
GVF_SHA=$(awk '/^    gvforwarder:/ { f=1; next } f && /sha256:/ { print $2; exit }' "$PINS")
GVP_SHA=$(awk '/^    gvproxy-windowsgui\.exe:/ { f=1; next } f && /sha256:/ { print $2; exit }' "$PINS")
$PODMAN build --target fetcher --file build/Dockerfile.rootfs \
  --build-arg "GVTV_TAG=$GVTV_TAG" \
  --build-arg "GVTV_GVFORWARDER_SHA256=$GVF_SHA" \
  --build-arg "GVTV_GVPROXY_EXE_SHA256=$GVP_SHA" .
```

Expected: exit 0, both `sha256sum -c` checks print OK in the layer log. If GitHub-release downloads stall in this session, mark this check "deferred to CI" in the PR Test plan — do not claim it ran.

- [ ] **Step 2: Negative — corrupted sha must fail**

Same build with `GVF_SHA` replaced by 64 zeros:

```bash
$PODMAN build --target fetcher --file build/Dockerfile.rootfs \
  --build-arg "GVTV_TAG=$GVTV_TAG" \
  --build-arg "GVTV_GVFORWARDER_SHA256=0000000000000000000000000000000000000000000000000000000000000000" \
  --build-arg "GVTV_GVPROXY_EXE_SHA256=$GVP_SHA" . && echo "GUARD-BROKEN" || echo "GUARD-BITES"
```

Expected: `GUARD-BITES` (sha256sum -c fails the layer).

- [ ] **Step 3: Clean session podman storage**

```bash
chmod -R u+w "$SP/podman" 2>/dev/null; rm -rf "$SP/podman"
```

---

### Task 5: CLAUDE.md live truth + single commit

**Files:**
- Modify: `CLAUDE.md` (repo-layout renovate.json line, repo-layout ci.yml line, new Status paragraph)

**Interfaces:**
- Consumes: staged changes from Tasks 1-3.
- Produces: the single implementation commit on `chore/renovate-repair`.

- [ ] **Step 1: Repo-layout renovate.json line**

Replace:

```
├── renovate.json                                   ← 3 streams: gomod, alpine+go-apk lockstep, gvisor-tap-vsock release
```

with:

```
├── renovate.json                                   ← 4 streams: gomod, alpine-3.23 digest + go-apk lockstep, gvisor-tap-vsock tag (sha256 manual, CI-guarded), github-actions
```

- [ ] **Step 2: Repo-layout ci.yml line**

In the ci.yml layout line, replace `build verify, race` with `build verify, upstream-pins verify, race`.

- [ ] **Step 3: New Status paragraph**

Insert after the "**Public-surface polish (post-v0.2.1)**" paragraph:

```
**Renovate activated** as of 2026-07-28. The Mend Renovate GitHub App was never installed during Phase B — `renovate.json` sat inert for 11 weeks with zero bot PRs, discovered when the stale `go=1.25.9-r0` apk pin broke the release build (fixed in PR #30). App installed 2026-07-28 (Scan and Alert), dependency dashboard enabled, alpine digest manager fenced to the 3.23 line to match the repology `alpine_3_23/go` apk manager (RR-D-3), and CI gained an always-on fetcher-stage build verifying `build/upstream-pins.yaml` tag/sha coherence — Renovate bumps tags only, sha256 stays manual (RR-D-1). Spec: `docs/superpowers/specs/2026-07-28-renovate-repair-design.md`.
```

- [ ] **Step 4: Review staged diff and commit**

Run: `git add CLAUDE.md && git diff --cached --stat`
Expected: exactly 4 files — `renovate.json`, `build/upstream-pins.yaml`, `.github/workflows/ci.yml`, `CLAUDE.md`.

```bash
git commit -m "renovate repair: fence alpine digest manager to 3.23 line, enable dependency dashboard, add always-on CI fetcher-stage guard for upstream-pins tag/sha coherence, fix upstream-pins header claim and CLAUDE.md drift"
```

---

### Task 6: Push, PR, CI, merge (GATED on user authorization)

**Files:**
- No edits.

**Interfaces:**
- Consumes: branch `chore/renovate-repair` with spec + plan + implementation commits.
- Produces: merged PR on `main`; post-merge Renovate dashboard verification.

- [ ] **Step 1: Freshness check**

Run: `git fetch origin && git log origin/main..HEAD --oneline`
Expected: origin/main still at `ec6afa1`; 3 commits ahead (spec, plan, implementation).

- [ ] **Step 2: ASK THE USER for push authorization** (quote branch + commits)

- [ ] **Step 3: Push and open PR (after yes)**

```bash
git push -u origin chore/renovate-repair
gh pr create --title "renovate repair: 3.23 digest fence, dependency dashboard, CI upstream-pins guard" --body "$(cat <<'EOF'
## Summary
- Renovate app is now actually installed (2026-05-10 config was never executed — the app was missing; zero bot PRs in 11 weeks). Before its first scheduled run this PR fixes the two config bugs that would have produced broken bot PRs: the alpine digest customManager tracked `latest` (= 3.24 line) while the go apk manager hardcodes repology `alpine_3_23/go` — a grouped "lockstep" PR would merge green and produce an image where `apk add go=1.25.10-r0` fails; digest manager now fenced to `3.23` (RR-D-3).
- gvisor-tap-vsock bumps: Renovate can only bump the `tag:` line of `build/upstream-pins.yaml` (sha256 stays manual), which was CI-invisible and release-breaking. CI now builds the Dockerfile fetcher stage on every run, so tag/sha mismatch fails the PR (RR-D-1). upstream-pins.yaml header now states the real contract.
- Dependency dashboard enabled (RR-D-2) as a standing bot heartbeat; CLAUDE.md updated (4 streams, new Status paragraph, ci.yml step list).

## Test plan
- [ ] `renovate.json` parses (`python3 -m json.tool`)
- [ ] Local fetcher-stage build with current v0.8.8 pins passes (same commands as the new CI step)
- [ ] Local negative test: corrupted gvforwarder sha256 fails the fetcher build (guard bites)
- [ ] PR CI exercises the new step against current pins
- [ ] Post-merge: "Dependency Dashboard" issue appears (bot alive, config parses)
EOF
)"
```

Adjust Test plan checkboxes to what actually ran (Task 4 deferred ⇒ say so).

- [ ] **Step 4: Wait for CI, merge**

Run: `gh pr checks --watch` → expect `ci` SUCCESS → `gh pr merge --merge` (squash/rebase disabled repo-wide). Resolve review threads first if any.

- [ ] **Step 5: Post-merge**

```bash
git checkout main && git pull --ff-only && git branch -d chore/renovate-repair
```

Then watch for the "Dependency Dashboard" issue (bot's first run after merge); if the bot files a config-error issue instead, fix forward.
