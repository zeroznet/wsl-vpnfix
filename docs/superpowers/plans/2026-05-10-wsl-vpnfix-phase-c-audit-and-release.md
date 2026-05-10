<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix Phase C — Audit, Threat Model, and Master-Spec Rebase Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the documentation backbone left open after Phase B as three docs-only PRs, then close Phase C with the `v0.2.0` tag. Artifact 1 lifts master spec section 3 into a standalone `docs/THREAT-MODEL.md` and replaces section 3 with a one-line pointer in the same PR (no duplicate-state interval). Artifact 2 ships `docs/SECURITY-AUDIT.md` as a short catalogue grouped by area. Artifact 3 rebases the master spec (folds Phase B addendum amendments, collapses section 8 open items, drops the verbose F-007 example), deletes the addendum file, and lands the `CLAUDE.md` and `TODO.md` housekeeping in the same PR. The tag triggers the existing release pipeline; a small post-tag PR updates `CLAUDE.md`'s Status section.

**Architecture (per `docs/superpowers/specs/2026-05-10-wsl-vpnfix-phase-c-design.md`):**

Three feature branches, three PRs, one tag, one post-tag PR. Branch names follow the existing repo pattern (`<area>/<short>`): `phase-c/threat-model`, `phase-c/audit-doc`, `phase-c/spec-rebase`, `phase-c/claude-md-status`. Branch protection on `main` requires the `ci` status check (which on a markdown-only diff still runs `gofmt`, `go vet`, `go mod verify`, `govulncheck`, unit and integration tests, and the build verify — all expected to pass without changes) plus conversation resolution on the PR. Each PR's body explains the diff in 2–3 bullets; the spec is the long-form source of truth and is not duplicated into PR descriptions.

**Tech Stack:** Markdown only. No Go changes. No build pipeline changes. `release.yml` is unchanged from Phase B and triggers automatically on the `v0.2.0` tag push, building `out/wsl-vpnfix-0.2.0.tar.gz` (binary identical to v0.1.0 except for `-X main.version=v0.2.0`), `SHA256SUMS`, and `upstream-pins.yaml`.

**Out of scope for Phase C (per spec section 8):**

Default-route persistence to disk, `wsl-vpnfixctl` debug subcommand, fault-injection integration tests, cosign signing / SBOM / SLSA, README copy edits / `install-wslvpnfix.ps1` changes / logo, strict `govulncheck` re-enable. The first two move to `TODO.md` Backlog as part of Task C3; the rest stand as recorded decisions.

---

## File Structure

Files created, modified, or deleted in this phase:

```
wsl-vpnfix/
├── CLAUDE.md                                                 ← C3 (drop addendum refs), C5 (status section update)
├── TODO.md                                                   ← C3 (Backlog adds, Later removal, Now removal)
├── docs/
│   ├── THREAT-MODEL.md                                       ← C1 (NEW; lifted from master spec section 3)
│   ├── SECURITY-AUDIT.md                                     ← C2 (NEW; short catalogue grouped by area)
│   └── superpowers/
│       ├── specs/
│       │   ├── 2026-05-08-wsl-vpnfix-design.md               ← C1 (section 3 → pointer), C3 (section 4/6/8/frontmatter rebase)
│       │   └── 2026-05-09-wsl-vpnfix-phase-b-design.md       ← C3 (DELETED)
│       └── plans/
│           └── 2026-05-10-wsl-vpnfix-phase-c-audit-and-release.md   ← this file
```

Boundaries:

- `docs/THREAT-MODEL.md` is the single source of truth for adversaries, trust boundaries, assets, audit checklist, findings format, and re-audit cadence. The master spec section 3 becomes a one-line pointer; no fact appears in both places.
- `docs/SECURITY-AUDIT.md` references the threat model for severity context (severity is implicit in the area grouping per Phase C decision C-D-3) but does not restate adversary or trust-boundary content.
- The master spec rebase is in-place, in one PR. Section 3 was already replaced with a pointer in C1; the rebase only touches sections 4, 6, 8, and the frontmatter status line.
- The Phase B addendum is **deleted**, not archived. Git history is the archive (`git show 590d6cc:docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md` recovers it any time).

This is the unit-of-PR layout: C1 → PR1, C2 → PR2, C3 → PR3, C4 = tag (no PR, just `git tag` + `git push --tags`), C5 → PR4 (post-tag).

---

## Task C1: `docs/THREAT-MODEL.md` lift + master spec section 3 → pointer (PR1)

**Files:**
- Create: `docs/THREAT-MODEL.md`
- Modify: `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` (section 3 only)
- Branch: `phase-c/threat-model`

The lift is mostly mechanical. Three substantive edits during the lift:

1. Section 3.5 (Findings format) is **rewritten** to the short-catalogue style used in C2's audit doc: `F-NNN <title> — <risk one-liner>; status: <commit | tracked-in-TODO | fixed-in-v0.1.0>`. The verbose 7-field example F-007 from the master spec does not survive into the threat model; F-007 is catalogued in `docs/SECURITY-AUDIT.md` instead, with its real "reversed in `0893652`" status. A short paragraph after the format example explains why severity is implicit in the area grouping rather than a separate field.
2. Section 3.6 (Audit cadence) drops the sentence "Initial audit: before tagging `v1.0.0`. No public release before the audit doc lands." That gate was already broken when v0.1.0 shipped public; the threat model reflects current reality. Re-audit triggers stand: any PR touching `internal/netfilter`, `internal/process`, `internal/wsl`, or the build pipeline; any pinned-upstream major bump; any Alpine base bump.
3. Section 3.1 (Adversaries) row 6 ("Malicious user's own VPN client") gets a one-line refresh noting that this is the production deployment shape on Cisco AnyConnect, validated in `docs/smoke-2026-05-10-workpc-vpn.md`. The threat model is no longer hypothetical for that adversary.

After the lift, master spec section 3 (every subsection 3.1 through 3.7) is replaced with a one-line pointer. No duplicated content per workspace `CLAUDE.md` guideline #10 ("Lean Files — No Duplicate State").

- [ ] **Step 1: Create the feature branch**

```bash
git fetch origin
git checkout -b phase-c/threat-model origin/main
```

Expected: branch created from latest `main`.

- [ ] **Step 2: Write `docs/THREAT-MODEL.md`**

Path: `docs/THREAT-MODEL.md` (repo root).

Source: `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` sections 3.1 through 3.7 (current `HEAD` of `main`). Lift verbatim except for the three substantive edits described above.

Required structure (use exactly these top-level headings to keep the lift verifiable by grep):

```markdown
<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix — Threat Model

**Status:** Living reference, last reviewed 2026-05-10.

A standalone reference for the adversaries, trust boundaries, and audit scope of `wsl-vpnfix`. Lifted from `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` section 3 on 2026-05-10 to give the security audit catalogue (`docs/SECURITY-AUDIT.md`) a stable target to reference.

## 1. Adversaries

[Lift master spec section 3.1 verbatim. Update row 6 ("Malicious user's own VPN client") to add a sentence noting this is the production shape validated on Cisco AnyConnect in docs/smoke-2026-05-10-workpc-vpn.md — not hypothetical.]

## 2. Trust boundaries

[Lift master spec section 3.2 verbatim, including the ASCII diagram.]

## 3. Assets

[Lift master spec section 3.3 verbatim.]

## 4. In-scope audit checklist

[Lift master spec section 3.4 verbatim. The systemd-service-unit hardening bullet was already removed by the Phase B addendum amendment; the lifted version reflects the post-addendum state, not the original. The orchestrator-owns-its-own-privilege-model paragraph from the addendum amendment replaces it.]

## 5. Findings format

Findings are catalogued in `docs/SECURITY-AUDIT.md` as one-line entries grouped by area:

```
F-NNN <title> — <risk one-liner>; status: <commit-sha | tracked-in-TODO | fixed-in-v0.1.0>
```

Severity is implicit in the area grouping (`orchestrator`, `netfilter`, `supply-chain`, `wsl-interop`, `build`, `known-gaps`) rather than a separate field. The 7-field per-finding template in the original master spec was performative ceremony for findings that are mostly "small bug, fixed in commit X"; the short catalogue gives attention proportional to risk and survives the realistic maintenance cadence.

## 6. Re-audit cadence

Re-audit triggers:

- Any PR touching `internal/netfilter`, `internal/process`, `internal/wsl`, or the build pipeline.
- Any pinned-upstream major version bump (`gvisor-tap-vsock`, Alpine base, Go toolchain).
- Any Alpine base image bump beyond patch.

External eyes: the threat model and audit doc are public artifacts open to community comment. The audit doc is the trust artifact; the project does not say "trust us".

## 7. Out of scope

[Lift master spec section 3.7 verbatim.]
```

Substantive content for each section comes from the master spec at `HEAD`. Read `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` lines 135–248 (sections 3.1 through 3.7) and copy in. The three required edits are flagged inline above.

- [ ] **Step 3: Verify the lift produced complete content**

```bash
test -f docs/THREAT-MODEL.md && \
  grep -c '^## ' docs/THREAT-MODEL.md
```

Expected: `7` (seven top-level numbered sections: Adversaries, Trust boundaries, Assets, In-scope audit checklist, Findings format, Re-audit cadence, Out of scope).

```bash
grep -E '(Cisco AnyConnect|smoke-2026-05-10-workpc-vpn)' docs/THREAT-MODEL.md
```

Expected: at least one hit (the adversary class 6 refresh).

```bash
grep -F 'No public release before audit doc lands' docs/THREAT-MODEL.md && \
  echo "FAIL: stale gate sentence still present" || echo "OK: stale gate sentence removed"
```

Expected: `OK: stale gate sentence removed`.

- [ ] **Step 4: Replace master spec section 3 with a one-line pointer**

Open `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md`. Locate the section that currently reads:

```markdown
## 3. Threat model & audit scope

### 3.1 Adversaries

[... through end of 3.7 ...]
```

Replace the entire block (every line from `## 3. Threat model & audit scope` through the closing `---` separator that precedes `## 4. Build & release pipeline`) with:

```markdown
## 3. Threat model & audit scope

See `docs/THREAT-MODEL.md`. Lifted from this section on 2026-05-10; this header survives only as a section number anchor for the rest of the document.

---
```

The three lines between the heading and the separator are the entire content of section 3 going forward. Findings catalogue lives in `docs/SECURITY-AUDIT.md`, not here.

- [ ] **Step 5: Verify the master spec section 3 is now a pointer**

```bash
sed -n '/^## 3\. Threat model/,/^## 4\./p' \
  docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md | wc -l
```

Expected: a small number (≤ 8 lines). If it returns 100+, the old verbose section is still present.

```bash
grep -c '^### 3\.' docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md
```

Expected: `0` (no more `### 3.1`, `### 3.2`, etc. — they all live in `docs/THREAT-MODEL.md`).

- [ ] **Step 6: Commit**

```bash
git add docs/THREAT-MODEL.md \
        docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md
git commit -m "threat-model: lift master spec section 3 to docs/THREAT-MODEL.md, replace section with one-line pointer (no duplicate state per workspace guideline #10)"
```

Expected: one new file, one modified file, single-line commit message per workspace `CLAUDE.md` rule.

- [ ] **Step 7: Push and open PR**

```bash
git push -u origin phase-c/threat-model
gh pr create --title "threat-model: extract docs/THREAT-MODEL.md from master spec section 3" --body "$(cat <<'EOF'
## Summary

- Lifts master spec section 3 (adversaries, trust boundaries, assets, audit checklist, findings format, cadence, out-of-scope) into a standalone `docs/THREAT-MODEL.md`.
- Master spec section 3 reduced to a one-line pointer; no duplicate state per workspace `CLAUDE.md` guideline #10.
- Refreshes adversary class 6 with the Cisco AnyConnect smoke-validated production shape from `docs/smoke-2026-05-10-workpc-vpn.md`.
- Drops the "no public release before audit doc lands" cadence sentence (already broken when v0.1.0 shipped).
- Rewrites section 3.5 findings format to the short-catalogue style used by `docs/SECURITY-AUDIT.md` (PR2).

## Test plan

- [ ] Threat model reads cleanly end to end.
- [ ] `grep -c '^## ' docs/THREAT-MODEL.md` returns 7.
- [ ] Master spec section 3 is a one-line pointer (≤ 8 lines between `## 3.` and `## 4.`).
- [ ] No `### 3.x` subsections remain in the master spec.
EOF
)"
```

Expected: PR URL printed.

- [ ] **Step 8: Wait for CI green and resolve any review threads**

```bash
gh pr view --json mergeable,mergeStateStatus,statusCheckRollup --jq '{mergeable, mergeStateStatus, ci: .statusCheckRollup[] | select(.name=="ci") | {status, conclusion}}'
```

Wait until `mergeStateStatus` is `CLEAN` and `ci.conclusion` is `SUCCESS`. If the Codex bot leaves a review comment, address each finding (apply fix in a new commit on the same branch, push, reply to the thread acknowledging the fix, resolve the thread via `gh api graphql -f query='mutation { resolveReviewThread(input: {threadId: "<id>"}) { thread { isResolved } } }'`).

- [ ] **Step 9: Merge and clean up**

Merge requires explicit user authorization (auto-mode classifier blocks direct merges to `main`).

```bash
gh pr merge --merge --delete-branch
git checkout main
git pull --ff-only origin main
```

Expected: PR shows `MERGED`, branch deleted, local `main` advanced to the merge commit.

---

## Task C2: `docs/SECURITY-AUDIT.md` short catalogue (PR2)

**Files:**
- Create: `docs/SECURITY-AUDIT.md`
- Branch: `phase-c/audit-doc`

The audit doc is a short catalogue. One line per finding in this exact format:

```
F-NNN <title> — <risk one-liner>; status: <commit-sha | tracked-in-TODO | fixed-in-v0.1.0>
```

Findings are grouped by area. Severity is implicit in the area grouping — no separate severity field. Source material (everything below has a verifiable provenance):

| Source | What to extract |
|---|---|
| `docs/superpowers/plans/2026-05-08-wsl-vpnfix-phase-a-core-runtime.md` Self-Review section, "Code-review pass (2026-05-08) corrections" subsection | Findings C-1 through C-5 (TUNSETIFF MAC pinning, pgroup signaling for grandchild reaping, `CaptureAndDelDefaultRoutes` snapshot, MASQUERADE saddr CIDR scope per F-007 — note this one is later reversed, nft atomic split). |
| same Self-Review section, "Implementation-pass (2026-05-09) corrections" subsection | Findings C-6 through C-8 (`isDefaultDst` for `Dst=nil` OR `0.0.0.0/0`, pgroup-kill test PID-namespace adaptation, `.gitignore` anchor fix). |
| same Self-Review section, "Smaller corrections from the same pass" paragraph | Three smaller security-relevant findings (tightened `absPathRe` against argv smuggling, `autoGenMarker` + `WSL2_GATEWAY_IP` env override against silently misdirected NAT, whitespace-only env-value rejection). The DNS-test tautology fix and the `debugInt`/`boolStr` dedup are code-quality housekeeping, not audit material — skip them. |
| `docs/smoke-2026-05-10.md` "Bug fixes that came out of this smoke run" table (lines 28–37), excluding `0893652` which is catalogued separately as F-007 below | Findings for commits `974e9b4` (gvproxy 9P → NTFS DrvFs page-fault workaround), `2b54529` (`-ssh-port=-1` to disable gvproxy default SSH listener), `e30d045` (`-config` YAML workaround for upstream `-listen-stdio` regression). |
| same smoke note (commit `0893652`) plus `docs/smoke-2026-05-10-workpc-vpn.md` "Master-spec implications (work-PC validations)" section (lines 30–32) | One catalogue entry **reserved as F-007**: MASQUERADE saddr-scope drop = the F-007 reversal. The work-PC smoke note confirms the reversal is required in production (sibling distros sit in `172.x.x.x`, outside `192.168.127.0/24`); cite both smoke notes as evidence on the F-007 entry. The Phase A C-4 finding originally added the saddr scope per the master spec's hypothetical F-007; reality reversed it. Do not catalogue C-4 separately — its entry IS F-007 with the inverted conclusion. |
| `TODO.md` Backlog | Two known gaps (default-route persistence, `wsl-vpnfixctl` debug subcommand) — added to `TODO.md` Backlog in Task C3. The audit doc references them with status `tracked-in-TODO`. |
| `CLAUDE.md` Status section + `TODO.md` Backlog "Re-enable strict `govulncheck`" bullet | Two tracked workarounds (govulncheck non-blocking pending alpine apk Go 1.25.10, gvproxy v0.8.8 stdio regression workaround). |

Total expected count: 7 (Phase A C-1, C-2, C-3, C-5, C-6, C-7, C-8 — C-4 is folded into F-007) + 3 (smaller A-pass) + 3 (B3 follow-on excluding `0893652`) + 1 (F-007 = `0893652` reversal, narrating both Phase A C-4 reasoning and production reality) + 2 (known gaps) + 2 (tracked workarounds) = **18 findings**.

- [ ] **Step 1: Create the feature branch**

```bash
git fetch origin
git checkout -b phase-c/audit-doc origin/main
```

Expected: branch created from latest `main` (which now includes C1's threat model).

- [ ] **Step 2: Write `docs/SECURITY-AUDIT.md`**

Required structure:

```markdown
<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix — Security Audit

**Status:** Living catalogue, last reviewed 2026-05-10. Threat model: `docs/THREAT-MODEL.md`.

A short-catalogue audit of `wsl-vpnfix` covering the Phase A self-review corrections, Phase B follow-on fixes uncovered by smoke testing, the F-007 reversal, currently-tracked gaps, and tracked workarounds blocked on upstream movement. Format per `docs/THREAT-MODEL.md` section 5: one line per finding, severity implicit in the area grouping.

## Orchestrator

- F-001 [title] — [risk]; status: [commit / tracked / fixed].
- F-002 ...
- ...

## Netfilter

- F-NNN ...

## Supply chain

- F-NNN ...

## WSL interop

- F-NNN ...

## Build

- F-NNN ...

## Known gaps

- F-NNN Default-route persistence to disk for crash-safe recovery — orchestrator deletes the original WSL2 default route at init; mid-init crash leaves siblings without networking until `wsl --shutdown`; status: tracked-in-TODO (Backlog).
- F-NNN `wsl-vpnfixctl` debug subcommand — no built-in `status` / `dump-config` / `verify-pins` for support diagnostics; status: tracked-in-TODO (Backlog).

## Tracked workarounds

- F-NNN `govulncheck` non-blocking in CI — Go 1.25.9 has 2 stdlib CVEs (`GO-2026-4971`, `GO-2026-4918`) fixed in 1.25.10; alpine apk pin blocks the bump; both low exposure for this threat model (Linux-only target, gvproxy-trusted HTTP/2 peer); status: tracked-in-TODO (Backlog) until alpine ships Go 1.25.10.
- F-NNN gvproxy v0.8.8 `-listen-stdio` regression workaround — upstream parses the CLI flag but never wires it into `config.Interfaces.Stdio`; we emit a `-config` YAML to set it via `interfaces.stdio`; status: fixed-in-v0.1.0 (commit `e30d045`), re-evaluate when upstream cuts the next release.

## Re-audit triggers

See `docs/THREAT-MODEL.md` section 6.
```

Number findings sequentially (F-001, F-002, ...) within and across area groups. Reserve **F-007** specifically for the MASQUERADE-saddr-scope finding, to preserve the historical number used by the master spec example and the smoke notes.

For each finding, the title should be a 3–6 word noun phrase identifying the concrete defect (not the fix). The risk one-liner should describe what could go wrong if the defect were unfixed, in present tense, ≤ 100 characters. Examples (non-prescriptive — write the actual ones from the source material):

```
F-001 TUNSETIFF MAC silently ignored — kernel auto-assigns random MAC, breaks gvproxy DHCP static lease lookup; status: fixed-in-v0.1.0 (Phase A C-1).
F-007 MASQUERADE saddr CIDR scope dropped sibling-distro packets — sibling distros sit in 172.x.x.x outside 192.168.127.0/24, so qualifier scope skipped them and gvproxy could not reply; status: reversed in 0893652 (Phase B B3 smoke), Phase A C-4 fix was the wrong shape.
```

The F-007 entry **must** narrate both the original Phase A C-4 reasoning (why we thought the scope was needed) and the production reality (why it was wrong) so the lesson survives the master-spec rebase that removes the example from section 3.5.

- [ ] **Step 3: Verify finding count and format**

```bash
grep -c '^- F-' docs/SECURITY-AUDIT.md
```

Expected: `18` (matches the source-material breakdown above; `0893652` is catalogued only once as F-007, and Phase A C-4 is folded into F-007 because C-4's fix is exactly what F-007 reverses).

```bash
grep -E '^- F-[0-9]{3} .+ — .+; status: .+\.$' docs/SECURITY-AUDIT.md | wc -l
```

Expected: `18` (every line matches the required `F-NNN <title> — <risk>; status: <...>` pattern).

```bash
grep '^- F-007 ' docs/SECURITY-AUDIT.md
```

Expected: a line whose status field contains both `0893652` and the word `reversed` or `reversal`. Confirms the finding number and the substance.

```bash
grep -c '^## ' docs/SECURITY-AUDIT.md
```

Expected: `8` (Orchestrator, Netfilter, Supply chain, WSL interop, Build, Known gaps, Tracked workarounds, Re-audit triggers).

- [ ] **Step 4: Commit**

```bash
git add docs/SECURITY-AUDIT.md
git commit -m "audit: docs/SECURITY-AUDIT.md short catalogue — 7 Phase A corrections (C-4 folded into F-007) + 3 smaller A-pass + 3 B3 follow-on + F-007 reversal (commit 0893652) + 2 known gaps + 2 tracked workarounds (18 findings, severity implicit in area grouping)"
```

- [ ] **Step 5: Push and open PR**

```bash
git push -u origin phase-c/audit-doc
gh pr create --title "audit: docs/SECURITY-AUDIT.md short catalogue (18 findings)" --body "$(cat <<'EOF'
## Summary

- New `docs/SECURITY-AUDIT.md`, short-catalogue style per `docs/THREAT-MODEL.md` section 5.
- 18 findings: 7 Phase A self-review corrections (C-1, C-2, C-3, C-5, C-6, C-7, C-8 — C-4 is folded into F-007 because C-4's saddr-scope fix is exactly what F-007 reverses), 3 smaller security-relevant Phase A corrections, 3 Phase B follow-on fixes from smoke testing (974e9b4, 2b54529, e30d045), F-007 reversal (commit 0893652) as its own catalogued finding narrating both the Phase A C-4 reasoning and the production reality that contradicted it, 2 known gaps (default-route persistence + `wsl-vpnfixctl`), 2 tracked workarounds (govulncheck non-blocking + gvproxy v0.8.8 stdio regression).
- Severity implicit in area grouping (orchestrator, netfilter, supply-chain, wsl-interop, build, known-gaps, tracked-workarounds) — no separate severity field.

## Test plan

- [ ] `grep -c '^- F-' docs/SECURITY-AUDIT.md` returns 18.
- [ ] Every finding line matches `F-NNN <title> — <risk>; status: <...>` pattern.
- [ ] F-007 entry narrates both Phase A C-4 reasoning and production reversal; commit `0893652` is in its status field.
- [ ] No leaked Slovak in the doc text.
EOF
)"
```

- [ ] **Step 6: Wait for CI green, resolve threads, merge**

Same rituals as C1 Steps 8–9. Merge requires explicit user authorization.

---

## Task C3: Master-spec rebase + addendum delete + housekeeping (PR3)

**Files:**
- Modify: `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` (sections 4, 6, 8, frontmatter status line)
- Delete: `docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md`
- Modify: `CLAUDE.md` (drop addendum bootstrap line + addendum-overrides sentence)
- Modify: `TODO.md` (Backlog adds, Later removal, Now removal)
- Branch: `phase-c/spec-rebase`

This is the largest of the three PRs but still all docs. The Phase B addendum's "Master-spec amendments" table (`docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md` section 2) is the authoritative diff to apply. Read it before starting; the steps below assume it is the source of truth for what each section 4 subsection should look like after the rebase.

- [ ] **Step 1: Create the feature branch**

```bash
git fetch origin
git checkout -b phase-c/spec-rebase origin/main
```

- [ ] **Step 2: Section 4 rebase — apply Phase B addendum amendments in place**

Open both files side by side. For each row in the addendum's section 2 table, apply the amendment to the master spec body:

| Master spec subsection | Edit |
|---|---|
| 2.6 Rootfs contents | Drop systemd service unit reference. Add `[boot] command=/sbin/wsl-vpnfix` mechanism description. Add `/sbin/init` symlink note (kept as defensive cover, no-ops at runtime because `os.Getpid() != 1`). Final rootfs contents per addendum: `/sbin/wsl-vpnfix`, `/sbin/wsl-gvforwarder`, `/etc/wsl-vpnfix/wsl-gvproxy.exe`, `/etc/wsl-vpnfix/checksums`, `/etc/wsl.conf` (with `command=/sbin/wsl-vpnfix`), `/sbin/init` symlink, `LICENSE`, plus busybox, Alpine baselayout, `nftables`, `iproute2`, `ca-certificates`. No `bash`, no `curl`, no `wget`, no compilers, no SSH. |
| 4.2 Build steps | Drop the SBOM step (formerly 5b). Steps 1–5a stand. |
| 4.3 Reproducibility | Keep the build flags (`-trimpath`, `-buildid=`, `SOURCE_DATE_EPOCH`, `gzip -n`, sorted tar, fixed owner/mode). Drop the dedicated `reproducibility.yml` workflow reference. |
| 4.4 Signing | Delete the entire subsection. Cosign keyless dropped per Phase B D-2. |
| 4.5 CI / CD | Workflows reduce to `ci.yml` and `release.yml`, both on `ubuntu-24.04`. `release.yml` does not need `id-token: write`. `ci.yml` runs `gofmt -l .`, `go vet ./...` plus `go vet -tags=integration ./...`, `go mod verify`, `govulncheck ./...`, unit + integration tests, and a build verify. Renovate config (`renovate.json`) at the repo root with three customManager-driven streams. |
| 4.6 Release artifacts | Final list: `wsl-vpnfix-X.Y.Z.tar.gz`, `SHA256SUMS`, `upstream-pins.yaml`. Drop `.sig`, `.pem`, `.cdx.json`, `SHA256SUMS.sig`. |
| 4.7 Versioning | **Rewrite per Phase C C-D-1**, not per Phase B addendum. Final text: SemVer applies, the project currently sits on the 0.x line, no API or behavior stability commitment yet. There is no v1.0 audit gate. Phase A shipped v0.1.0; Phase C closes with v0.2.0. Bump policy: patch = bug fix / security fix in our code / upstream pin bump; minor = new env var / opt-in feature / Alpine major bump / documentation milestone (e.g. audit doc landing); major = breaking config change / dropping `wsl --import` compatibility / anything that changes user-observable behavior. |
| 4.8 User update workflow | Replace the `cosign verify-blob` block with `sha256sum -c SHA256SUMS` (the PowerShell `Get-FileHash` equivalent in `scripts/install-wslvpnfix.ps1` already shipped in v0.1.0). |

- [ ] **Step 3: Verify section 4 contains no stale signing references**

```bash
grep -nE 'cosign|SBOM|syft|SLSA|reproducibility\.yml|id-token: write' \
  docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md
```

Expected: zero output. If anything matches, those are unrebased remnants.

- [ ] **Step 4: Section 6 (Decisions table) — versioning + architectures rows**

Locate the decisions table. Update two rows:

```
| Versioning | SemVer; v0.x during audit, v1.0.0 after initial audit lands |
```

becomes

```
| Versioning | SemVer; currently 0.x line, no v1.0 audit gate; Phase C closes with v0.2.0 |
```

and

```
| Architectures | amd64 only at v1.0; ARM deferred |
```

becomes

```
| Architectures | amd64 only; ARM deferred |
```

Leave every other row unchanged.

- [ ] **Step 5: Section 8 (Open items) collapse**

Replace the entire section 8 body (every bullet from "Exact pinned Go toolchain version at v1.0 cut" through the partial-init teardown bullet) with this single paragraph (verbatim):

```
All original open items are resolved during Phase A (netlink library and nftables library choices, `wsl-vpnfixctl` folded into the same binary as flags then redistributed to TODO Backlog in Phase C), Phase B (Go toolchain pin, Alpine digest pin, `gvisor-tap-vsock` release tag pin, PID 1 vs systemd resolved as `[boot] command=` under WSL `/init`), or Phase C (default-route persistence redistributed to TODO Backlog, fault-injection integration tests dropped). `TODO.md` is the canonical home for any future open work; this section is intentionally short to avoid duplicating state.
```

- [ ] **Step 6: Frontmatter status line update**

Locate the frontmatter near the top of the master spec:

```
**Status:** Draft, pending user review
```

Replace with:

```
**Status:** Living spec, last rebased 2026-05-10 against Phase B addendum and Phase C decisions.
```

- [ ] **Step 7: Delete the Phase B addendum**

```bash
git rm docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md
```

Expected: file removed, staged for deletion.

- [ ] **Step 8: Update `CLAUDE.md` (drop addendum references)**

Open `CLAUDE.md`. Find the "Session bootstrap" section. The bootstrap-order list currently has a line referencing the Phase B addendum (item 4):

```
4. `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` — frozen master design contract; the Phase B addendum at `docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md` overrides it where they conflict, until the master-spec rebase lands in Phase C
```

Replace with:

```
4. `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` — living master design contract (last rebased 2026-05-10 to fold the Phase B addendum and Phase C decisions)
```

Renumber any subsequent items if needed (sanity-check by reading the surrounding paragraph). Do not touch the rest of `CLAUDE.md` in this step — the Status section update is Task C5, post-tag.

- [ ] **Step 9: Update `TODO.md` (Backlog adds, Later removal, Now removal)**

Open `TODO.md`. Three changes:

**Add to Backlog** (place under the existing Backlog bullets):

```
- [ ] **Default-route persistence to disk for crash-safe recovery.** Orchestrator's `CaptureAndDelDefaultRoutes` stashes the original WSL2 default route in RAM only; if `wsl-vpnfix` dies between `DelExistingDefaultRoute` and the new tap default install, restart cannot recover the original route — `RestoreRoutes` finds nothing to capture. Persist the snapshot to `/run/wsl-vpnfix/saved-routes.json` (or equivalent) so `systemctl restart wsl-vpnfix` mid-init does not require `wsl --shutdown`. Tracked as audit finding F-NNN. Brainstorm before plan; format / ACL / lifecycle on `wsl --unregister` are open design decisions.
- [ ] **`wsl-vpnfixctl` debug subcommand.** Same binary, separate flag set or subcommand. Operations: `status` (summary of active config, child PIDs, route + nft snapshot), `dump-config` (resolved env after validation), `verify-pins` (re-checks SHA-256 of bundled binaries against `/etc/wsl-vpnfix/checksums`). Tracked as audit finding F-NNN. Brainstorm before plan; subcommand vs flag-set is the open design call.
```

Replace `F-NNN` with the concrete finding numbers from `docs/SECURITY-AUDIT.md` (the two known-gaps entries).

**Remove from Later:**

The bullet currently in `TODO.md` Later that begins with "**Master spec rebase (post-Phase-B cleanup).**" — delete the entire bullet (it lands as part of this PR; the work no longer pending).

**Remove from Now:**

The bullet currently in `TODO.md` Now that begins with "**Phase C kickoff brainstorm.**" — delete the entire bullet (this plan supersedes it).

- [ ] **Step 10: Repo-wide grep sweep for dangling addendum references**

The plan file at `docs/superpowers/plans/2026-05-10-wsl-vpnfix-phase-c-audit-and-release.md` legitimately mentions the addendum filename when describing what to delete (this very step, the `git rm` step, the file-structure section, the PR body). That is historical execution record, not a live reference. Both sweeps below exclude `docs/superpowers/plans/` so the matches do not mask real dangling references elsewhere.

```bash
grep -rn '2026-05-09-wsl-vpnfix-phase-b-design' \
  --include='*.md' --include='*.json' --include='*.go' --include='*.sh' \
  --include='*.yml' --include='*.yaml' --include='*.ps1' \
  --exclude-dir='plans' \
  .
```

Expected: zero output. If any reference remains, fix it in this same commit.

```bash
grep -rn -E 'Phase B addendum|phase-b-design' \
  --include='*.md' --include='*.json' --include='*.go' --include='*.sh' \
  --include='*.yml' --include='*.yaml' --include='*.ps1' \
  --exclude-dir='plans' \
  .
```

Expected: matches only inside `CLAUDE.md` "history" or commit-log-flavored mentions that are still factually correct (e.g. "Phase B addendum was folded into the master spec on 2026-05-10"). Any pointer that suggests the addendum file still exists is a bug — fix it.

- [ ] **Step 11: Verify master spec is internally consistent**

```bash
grep -c '^### 3\.' docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md
```

Expected: `0` (still — section 3 was pointer-ified in C1).

```bash
grep -nE 'v1\.0\.0' docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md
```

Expected: zero matches, or matches only in the historical "Phase A shipped v0.1.0; Phase C closes with v0.2.0; v1.0 audit gate dropped" prose where the version is referenced as something that no longer applies. Any line that says "v1.0.0 ships only after audit lands" or similar is a remnant — delete it.

- [ ] **Step 12: Commit**

```bash
git add docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md \
        docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md \
        CLAUDE.md \
        TODO.md
git commit -m "spec: master-spec rebase folds Phase B addendum (sections 4 / 6 / 8 / frontmatter), drops v1.0 audit gate per Phase C C-D-1, deletes addendum file (git history is the archive); CLAUDE.md bootstrap drops addendum refs; TODO Backlog gains default-route persistence + wsl-vpnfixctl bullets"
```

- [ ] **Step 13: Push and open PR**

```bash
git push -u origin phase-c/spec-rebase
gh pr create --title "spec: master-spec rebase, addendum delete, TODO/CLAUDE.md housekeeping" --body "$(cat <<'EOF'
## Summary

- Master spec sections 4 / 6 / 8 / frontmatter rebased in place — folds the Phase B addendum amendments and applies Phase C C-D-1 (versioning: 0.x line, no v1.0 audit gate).
- `docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md` deleted (git history is the archive).
- `CLAUDE.md` Session-bootstrap line for the addendum dropped; pointer rewritten to "living master design contract, last rebased 2026-05-10".
- `TODO.md` Backlog gains default-route persistence and `wsl-vpnfixctl` bullets (cross-referenced to audit doc finding IDs); Later loses the rebase bullet; Now loses the Phase C kickoff bullet.
- Repo-wide grep sweep: zero dangling references to the deleted addendum.

## Test plan

- [ ] `grep -rn '2026-05-09-wsl-vpnfix-phase-b-design' --include='*.md' --include='*.json' --include='*.go' --include='*.sh' --include='*.yml' --include='*.yaml' --include='*.ps1' --exclude-dir='plans' .` returns nothing (the plan file in `docs/superpowers/plans/` legitimately mentions the addendum filename as a historical execution record and is excluded from the sweep).
- [ ] `grep -nE 'cosign|SBOM|syft|SLSA|reproducibility\.yml|id-token: write' docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md` returns nothing.
- [ ] Master spec section 8 is one paragraph (≤ 5 lines).
- [ ] Master spec frontmatter status reads "Living spec, last rebased 2026-05-10 …".
- [ ] No leaked Slovak.
EOF
)"
```

- [ ] **Step 14: Wait for CI green, resolve threads, merge**

Same rituals as C1 Steps 8–9. Merge requires explicit user authorization.

---

## Task C4: `v0.2.0` tag on `main` + release verification

**Files:**
- Modifies: GitHub Releases (via `release.yml` workflow trigger)
- Branch: none — tagged on `main`

The tag triggers `release.yml` (tag pattern `^v[0-9]+\.[0-9]+\.[0-9]+$`), which runs `build/pack.sh 0.2.0` and uploads the tarball, `SHA256SUMS`, and `upstream-pins.yaml` to a GitHub Release. Binary content is identical to v0.1.0 except for `-X main.version=v0.2.0` baked in.

- [ ] **Step 1: Confirm `main` is at the merged C3 commit**

```bash
git checkout main
git pull --ff-only origin main
git log --oneline -1
```

Expected: the most recent commit is the C3 merge (or its squash equivalent, depending on PR3 merge style). Check the message references the master-spec rebase.

- [ ] **Step 2: Tag and push**

Tagging `main` is a release action. **Requires explicit user authorization** before running — confirm with the user that C3 is fully reviewed and they want to cut v0.2.0.

```bash
git tag v0.2.0
git push origin v0.2.0
```

Expected: tag pushed; `release.yml` workflow run starts within seconds.

- [ ] **Step 3: Watch the release workflow to completion**

```bash
gh run watch --workflow=release.yml
```

Expected: workflow finishes with `SUCCESS`. If it fails, do not retry blindly — read the failing step's log, fix in a follow-on commit on `main` if it is a build-script bug, or delete the tag (`git push --delete origin v0.2.0`) and re-tag from a fixed commit.

- [ ] **Step 4: Verify GH Release artifacts**

```bash
gh release view v0.2.0 --json assets --jq '.assets[].name'
```

Expected: three lines, in any order:

```
SHA256SUMS
upstream-pins.yaml
wsl-vpnfix-0.2.0.tar.gz
```

```bash
gh release download v0.2.0 \
  --pattern 'SHA256SUMS' \
  --pattern 'wsl-vpnfix-0.2.0.tar.gz' \
  --pattern 'upstream-pins.yaml' \
  --dir /tmp/v020-verify
cd /tmp/v020-verify && sha256sum -c SHA256SUMS
```

Expected: `wsl-vpnfix-0.2.0.tar.gz: OK` and `upstream-pins.yaml: OK` (release pipeline writes checksums for both — downloading only the tarball would fail `sha256sum -c` on the missing file). If either reports `FAILED` or anything other than `OK`, the release is corrupt — investigate before announcing.

```bash
rm -rf /tmp/v020-verify
```

Per workspace `CLAUDE.md` Behavioral Guideline #6 (clean up temp files).

---

## Task C5: Post-tag `CLAUDE.md` Status section update (PR4)

**Files:**
- Modify: `CLAUDE.md` (Status section only)
- Branch: `phase-c/claude-md-status`

This is the last commit of Phase C. The Status section in `CLAUDE.md` currently describes Phase B as complete and Phase C as not started. After the tag lands, it should describe Phase C as complete with the same level of detail Phase B got.

- [ ] **Step 1: Create the feature branch**

```bash
git fetch origin
git checkout -b phase-c/claude-md-status origin/main
```

- [ ] **Step 2: Update `CLAUDE.md` Status section**

Open `CLAUDE.md`. Locate the Status section. Currently structured as:

```
**Phase A complete** as of 2026-05-09. ...

**Phase B complete** as of 2026-05-10. ...

**Phase C not started.** Remaining: ...
```

Replace the third paragraph (everything from "**Phase C not started.**" through the end of that paragraph, including the "Plan does not exist yet" sentence) with a "**Phase C complete**" paragraph mirroring the structure of the Phase B paragraph. Cover:

- Date complete (the date `v0.2.0` lands).
- The three artifacts (`docs/THREAT-MODEL.md`, `docs/SECURITY-AUDIT.md`, master-spec rebase) and their PR numbers.
- The `v0.2.0` tag and the artifacts it produced (binary identical to v0.1.0 except for the version string).
- The two items moved to TODO Backlog (default-route persistence, `wsl-vpnfixctl`) with one-line reasons.
- The one item dropped (fault-injection integration tests) with a one-line reason (cost exceeds value for this appliance shape).
- The CI follow-on still tracked: re-enable strict `govulncheck` once alpine apk ships Go 1.25.10.

Do not duplicate the audit doc's finding catalogue here. The Status section is a navigation aid, not a re-statement of the audit.

- [ ] **Step 3: Verify**

```bash
grep -c 'Phase C complete' CLAUDE.md
```

Expected: `1`.

```bash
grep -F 'Phase C not started' CLAUDE.md && \
  echo "FAIL: stale status line still present" || echo "OK: stale status line removed"
```

Expected: `OK: stale status line removed`.

- [ ] **Step 4: Commit, push, PR, merge**

```bash
git add CLAUDE.md
git commit -m "claude.md: Phase C closed status update — three docs landed (threat model + audit + master-spec rebase), v0.2.0 tagged, default-route persistence + wsl-vpnfixctl moved to TODO Backlog, fault-injection harness dropped"
git push -u origin phase-c/claude-md-status
gh pr create --title "claude.md: Phase C closed status update" --body "$(cat <<'EOF'
## Summary

- Status section reflects Phase C completion: three docs-only PRs landed, `v0.2.0` tagged, follow-ons redistributed.
- Mirrors the Phase A / Phase B paragraph structure for consistency.

## Test plan

- [ ] Status section reads cleanly; Phase A / Phase B paragraphs untouched.
- [ ] No leaked Slovak.
EOF
)"
```

Then wait for CI green, resolve any threads, and merge with explicit user authorization.

---

## Self-Review

**Spec coverage (against `docs/superpowers/specs/2026-05-10-wsl-vpnfix-phase-c-design.md`):**

| Spec section | Plan task(s) |
|---|---|
| 1. Scope (3 artifacts + v0.2.0 tag) | C1, C2, C3, C4 |
| 2. Decisions resolved (C-D-1 v0.2.0, C-D-2 docs-only, C-D-3 short catalogue) | C-D-1 → C3 step 2 (4.7 rewrite) + C4; C-D-2 → C3 step 9 (TODO Backlog adds) + scope of C1–C5 (no code); C-D-3 → C2 step 2 + C1 step 2 (3.5 rewrite) |
| 3. Artifact 1: THREAT-MODEL.md | C1 |
| 4. Artifact 2: SECURITY-AUDIT.md | C2 |
| 5. Artifact 3: Master-spec rebase | C3 (steps 2–11) |
| 6. TODO + housekeeping | C3 (steps 8–9) and C5 (CLAUDE.md status) |
| 7. Closing tag and release | C4 |
| 8. Out of scope | n/a (respected by scope of C1–C5 — no code, no signing, no README, no install.ps1, no logo) |
| 9. Success criteria | All six criteria are testable by the verification steps in C1 step 3+5, C2 step 3, C3 steps 3+10+11, C4 step 4, C5 step 3 |
| 10. After this spec → writing-plans → implementation | This plan is the writing-plans output; implementation is C1–C5 |

**Placeholder scan:** searched the plan for "TBD", "TODO", "implement later", "fill in details" — none in step content. Some occurrences of "TODO.md" reference the file itself, not a placeholder.

**Type / cross-task consistency:** Branch names match across tasks (`phase-c/threat-model`, `phase-c/audit-doc`, `phase-c/spec-rebase`, `phase-c/claude-md-status`). The audit doc's `F-NNN` finding numbers are used in `TODO.md` Backlog entries (C3 step 9) and in `CLAUDE.md` Status (C5 step 2) — the engineer is told to substitute the real numbers from `docs/SECURITY-AUDIT.md` after C2 lands. Verification commands cite exact files and exact expected outputs.

---

## After this plan

1. Plan reviewed by user. Changes round-trip until approved.
2. Execution proceeds via `superpowers:subagent-driven-development` (recommended; fresh subagent per task with two-stage review) or `superpowers:executing-plans` (inline, batched checkpoints).
3. Implementation lands as four PRs (C1, C2, C3, C5) plus one tag (C4) on `main`. Each PR is reviewed (Codex bot + maintainer) before merge; merge to `main` requires explicit user authorization in auto mode.
