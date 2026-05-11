<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix Phase C — Design

**Date:** 2026-05-10
**Status:** Completed 2026-05-10
**Author:** Robert Bopko, with Claude Opus 4.7 assistance

Closes the documentation backbone left open after Phase B: a standalone threat model, a security-audit catalogue, and a master-spec rebase that folds the Phase B addendum into the master spec. Phase C ships no new runtime code. It closes with the `v0.2.0` tag on `main`.

---

## 1. Scope

Phase C is documentation-only. Three artifacts, in the order they land:

1. `docs/THREAT-MODEL.md` — extracted from master spec section 3.
2. `docs/SECURITY-AUDIT.md` — short catalogue of findings, known gaps, and tracked workarounds.
3. Master-spec rebase — folds Phase B addendum amendments into the master spec, deletes section 3 (now a pointer to the standalone threat model), removes the verbose F-007 example finding (the real finding lives in the audit doc with status "reversed in `0893652`"), and archives the addendum by deletion.

No runtime change. No new Go code. The `v0.2.0` tag rebuilds the release tarball through the existing pipeline (binary identical to v0.1.0 except for the version string baked in via `-X main.version=$VERSION`).

---

## 2. Decisions resolved (Phase C kickoff brainstorm)

The `TODO.md` Now bullet flagged three open decisions for Phase C; the brainstorm resolved all three:

| # | Decision | Choice | Rationale |
|---|---|---|---|
| C-D-1 | Closing version tag | `v0.2.0` | The `v1.0.0` gate from master spec sections 3.6 and 4.7 ("no public release before audit doc lands") was already broken when v0.1.0 shipped publicly (commit `bfe44b8`, "deviates from plan's private-first; tracked for Phase C audit doc"). Staying on the 0.x line is more consistent with the released artifact and honestly signals "no API or behavior stability commitment yet". A minor bump (0.1 → 0.2) marks the documentation milestone without overcommitting to v1.0 semantics. |
| C-D-2 | Phase C scope shape | Docs-only | The four pre-v1.0 open items in master spec section 8 (default-route persistence, fault-injection integration tests, `wsl-vpnfixctl` debug subcommand, "exact pinned versions at v1.0 cut") were originally v1.0 gates. Without v1.0 as a deadline they redistribute: default-route persistence and `wsl-vpnfixctl` move to `TODO.md` Backlog with their own future design passes; fault-injection tests are dropped entirely (the cost of a netlink/netfilter mocking harness or a privileged failure-injection harness exceeds the value for a single-user appliance with no rollback semantics); the pinned-versions item is moot (Phase B already pinned them). |
| C-D-3 | Audit doc formality | Short catalogue | Master spec section 3.5 promised a 7-field per-finding template (Severity / Area / File / Repro / Risk / Fix / Status). For roughly fifteen findings, most of which are "small bug, fixed in commit X", the full template is performative ceremony. Phase C ships a one-line-per-finding catalogue: `F-NNN <title> — <risk one-liner>; status: <commit | tracked-in-TODO | fixed-in-v0.1.0>`, grouped by area. Section 3.5's example finding template is amended in the master-spec rebase to match the catalogue style actually used. |

---

## 3. Artifact 1 — `docs/THREAT-MODEL.md`

Lift master spec sections 3.1 through 3.7 into a standalone reference document. The lift is mostly mechanical:

- **3.1 Adversaries** → unchanged content. Optional minor refresh to adversary class 6 ("user's own VPN client") to note that this is the actual production deployment shape on Cisco AnyConnect, validated in `docs/smoke-2026-05-10-workpc-vpn.md` — the threat model is no longer hypothetical for that adversary.
- **3.2 Trust boundaries** → unchanged.
- **3.3 Assets** → unchanged.
- **3.4 In-scope audit checklist** → unchanged content. The Phase B addendum already deleted the systemd-service-unit hardening bullet and replaced it with the orchestrator's own privilege-model description; the lifted version reflects the post-addendum state, not the original.
- **3.5 Findings format** → rewritten to the short-catalogue style actually used in the audit doc. The verbose 7-field example is replaced with the one-line format and a brief paragraph explaining why severity is implicit in the area grouping rather than a separate field.
- **3.6 Audit cadence** → drop the "no public release before audit doc lands" sentence (already broken by v0.1.0). Re-audit triggers stand: any PR touching `internal/netfilter`, `internal/process`, `internal/wsl`, or the build pipeline; any pinned-upstream major bump; any Alpine base bump.
- **3.7 Out of scope** → unchanged.

After the lift, master spec section 3 is replaced by a one-line pointer to `docs/THREAT-MODEL.md`. No duplicated content per workspace `CLAUDE.md` guideline #10.

---

## 4. Artifact 2 — `docs/SECURITY-AUDIT.md`

Short catalogue, grouped by area (orchestrator, netfilter, supply-chain, wsl-interop, build, known-gaps). One line per finding in the format:

```
F-NNN <title> — <risk one-liner>; status: <commit-sha | tracked-in-TODO | fixed-in-v0.1.0>
```

Source material:

- **Phase A self-review (Corrections C-1..C-8)** in `docs/superpowers/plans/2026-05-08-wsl-vpnfix-phase-a-core-runtime.md`. Each correction becomes one finding. Severity is implicit in the area grouping; the explicit severity field is omitted per C-D-3.
- **Phase B follow-on commits** uncovered by the home-PC and work-PC smoke tests:
  - `974e9b4` — gvproxy.exe demand-paged over 9P faults with `EXCEPTION_IN_PAGE_ERROR` on Win 11 25H2; fixed by staging the binary into NTFS DrvFs (`/mnt/c/Users/Public/.wsl-vpnfix/`) before spawn.
  - `2b54529` — gvproxy default SSH listener on `:2222` conflicted with sibling-distro state; passed `ssh-port=-1` via the stdio URL to disable.
  - `e30d045` — gvproxy v0.8.8 has an upstream regression where the `-listen-stdio` CLI flag is parsed but never wired into `config.Interfaces.Stdio`; worked around by emitting a `-config` YAML.
  - `0893652` — MASQUERADE rule originally narrowed by saddr scope; sibling distros sit outside `192.168.127.0/24`, so the qualifier dropped their packets. The narrowing was Phase A finding F-007 (a hypothetical fix in the design); reality required the unscoped form.
- **F-007 reversal** — its own catalogued finding with status "reversed in `0893652`". The audit doc records both the original Phase A reasoning and why production reality contradicted it, so the lesson survives the spec edit that removes the example from section 3.5.
- **Known gaps** tracked in `TODO.md` Backlog:
  - Default-route persistence to disk for crash-safe recovery (master spec section 8 open item, redistributed to Backlog by C-D-2).
  - `wsl-vpnfixctl` debug subcommand (master spec section 8 open item, redistributed to Backlog by C-D-2).
- **Tracked workarounds:**
  - `govulncheck` non-blocking in CI: 2 stdlib CVEs in Go 1.25.9 fixed in 1.25.10; alpine apk pin blocks the bump. Re-enable strict mode once alpine apk ships 1.25.10.
  - gvproxy v0.8.8 stdio regression workaround documented above.

The doc closes with a "Re-audit triggers" pointer to `docs/THREAT-MODEL.md` section 3.6 to avoid duplication.

---

## 5. Artifact 3 — Master-spec rebase

In-place edits to `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md`:

- **Section 3** → replaced by a one-line pointer to `docs/THREAT-MODEL.md`. Subsections 3.1 through 3.7 are deleted in place; the standalone threat model is the single source of truth. The verbose 7-field example finding (F-007) is therefore no longer present in the master spec at all.
- **Section 4 (Build & release pipeline)** → rewritten to absorb the Phase B addendum amendments. The addendum's "Master-spec amendments" table is the authoritative diff:
  - 2.6 Rootfs contents → drop systemd service unit, add `[boot] command=` mechanism, add `/sbin/init` symlink note, add the PID-1-reaper-as-defensive-cover note.
  - 4.2 Build steps → drop the SBOM step.
  - 4.3 Reproducibility → keep the build flags, drop the dedicated `reproducibility.yml` workflow reference.
  - 4.4 Signing → deleted entirely (cosign keyless dropped per Phase B D-2).
  - 4.5 CI / CD → workflows reduce to `ci.yml` and `release.yml`, both on `ubuntu-24.04`; Renovate config noted; `id-token: write` removed.
  - 4.6 Release artifacts → final list trimmed to `wsl-vpnfix-X.Y.Z.tar.gz`, `SHA256SUMS`, `upstream-pins.yaml`.
  - 4.7 Versioning → rewritten: SemVer applies, the project currently sits on the 0.x line, no API or behavior stability commitment yet, no v1.0 audit gate.
  - 4.8 User update workflow → replace the `cosign verify-blob` block with `sha256sum -c SHA256SUMS` (or its PowerShell `Get-FileHash` equivalent in `scripts/install-wslvpnfix.ps1`).
- **Section 6 (Decisions)** → versioning row updated to match the new 4.7. Architectures row drops the "at v1.0" qualifier (becomes "amd64 only; ARM deferred").
- **Section 8 (Open items)** → all original bullets are either resolved, redistributed, or dropped:
  - Pinned Go toolchain, pinned Alpine digest, pinned gvisor-tap-vsock tag → resolved (Phase B pinned 1.25.9, the current Alpine digest, and v0.8.8 respectively).
  - `vishvananda/netlink` vs `mdlayher/netlink` → resolved (Phase A picked `vishvananda/netlink`).
  - `google/nftables` vs hand-rolled netlink expressions → resolved (Phase A picked `google/nftables`).
  - PID 1 vs systemd → resolved (Phase B addendum D-1 picked `[boot] command=` under WSL `/init`).
  - `wsl-vpnfixctl` → redistributed to `TODO.md` Backlog (one-line pointer in the spec).
  - Default-route persistence → redistributed to `TODO.md` Backlog (one-line pointer in the spec).
  - Fault-injection integration tests → dropped per C-D-2.
  Section 8 collapses to a short note such as: "All original open items resolved during Phase A, redistributed to `TODO.md` Backlog during Phase C, or explicitly dropped. `TODO.md` is the canonical home for any future open work; this section is intentionally short to avoid duplicating state."
- **Frontmatter status line** → updated from "Draft, pending user review" to "Living spec, last rebased 2026-05-10 against Phase B addendum and Phase C decisions".

After the rebase, `docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md` is **deleted**. Git history is the archive; no `archive/` directory.

A repo-wide grep sweep verifies no dangling references to the deleted addendum:

```sh
grep -rn "2026-05-09-wsl-vpnfix-phase-b-design" --include="*.md" --include="*.json" --include="*.go" --include="*.sh" .
```

Known reference sites that need updating in the same PR:

- `CLAUDE.md` bootstrap-order line for the addendum and the addendum-overrides-master-spec sentence.
- `TODO.md` Later bullet "Master spec rebase (post-Phase-B cleanup)" — removed (folded into Phase C and now done).

The sweep also looks for any other reference patterns — `Phase B addendum`, `phase-b-design`, etc. — to catch indirect mentions.

---

## 6. `TODO.md` and housekeeping

In `TODO.md`:

- **Backlog additions:**
  - Default-route persistence to disk for crash-safe recovery. Reason: orchestrator deletes WSL2's default route before installing the tap default; mid-init crash leaves siblings without networking until `wsl --shutdown`. Cross-references the audit doc finding.
  - `wsl-vpnfixctl` debug subcommand (`status`, `dump-config`, `verify-pins`). Same binary, separate flag set or subcommand; useful for support diagnostics.
- **Removals:**
  - "Master spec rebase (post-Phase-B cleanup)" Backlog bullet — done as part of Phase C.
  - The Phase C kickoff bullet itself, after this spec lands and writing-plans takes over.

In `CLAUDE.md`:

- Bootstrap order line referencing the Phase B addendum is dropped after the addendum is deleted in Artifact 3.
- The Status section is updated to reflect Phase C completion once the `v0.2.0` tag lands. This update is the last commit of Phase C, not part of the spec write itself.

---

## 7. Closing tag and release

`v0.2.0` is tagged on `main` after all three artifacts merge. The existing `release.yml` workflow handles:

- `build/pack.sh 0.2.0` produces `out/wsl-vpnfix-0.2.0.tar.gz` (binary identical to 0.1.0 except for `-X main.version=v0.2.0`).
- `SHA256SUMS` over the tarball plus `upstream-pins.yaml` (unchanged from v0.1.0 unless Renovate has bumped the gvisor-tap-vsock pin in the meantime).
- All three artifacts uploaded to the GitHub Release.

`scripts/install-wslvpnfix.ps1` continues to consume the latest release tag without modification.

---

## 8. Out of scope for Phase C

- Default-route persistence to disk. Tracked in `TODO.md` Backlog with a future brainstorm and spec.
- `wsl-vpnfixctl` debug subcommand. Tracked in `TODO.md` Backlog.
- Fault-injection integration tests for partial-init teardown. Dropped (C-D-2).
- Cosign keyless signing, SBOM, SLSA attestation. Phase B D-2 / D-3 stand.
- README copy edits, install.ps1 changes, logo. Public surface shipped at v0.1.0; Phase C touches only `docs/`, `TODO.md`, `CLAUDE.md`, and the master spec.
- Re-enabling strict `govulncheck` in CI. Tracked in `TODO.md` Backlog; depends on alpine apk shipping Go 1.25.10.

---

## 9. Success criteria

- `docs/THREAT-MODEL.md` exists; master spec section 3 is a one-line pointer to it; no duplicated content.
- `docs/SECURITY-AUDIT.md` catalogues 17 findings: seven Phase A self-review corrections (C-1..C-8, with C-4 folded into F-007 because the C-4 saddr-scope fix is exactly what F-007 reverses), three smaller A-pass corrections, three Phase B follow-on commits (excluding `0893652` which is F-007), the F-007 reversal, both known gaps from `TODO.md` Backlog, and the two tracked workarounds.
- Master spec is internally consistent: no addendum overrides, all section 4 edits applied in place, section 8 reflects current state only, section 3 is a pointer.
- `docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md` deleted; repo-wide grep returns no references.
- `TODO.md` Backlog has explicit bullets for default-route persistence and `wsl-vpnfixctl`; the "master spec rebase" bullet is gone; the Phase C kickoff bullet is gone.
- `CLAUDE.md` bootstrap order no longer references the deleted addendum.
- `v0.2.0` tagged on `main`; GitHub Release has tarball, `SHA256SUMS`, `upstream-pins.yaml`.

---

## 10. After this spec

1. User reviews this file. Changes round-trip through it until approved.
2. `superpowers:writing-plans` skill turns the approved spec into a step-by-step Phase C plan at `docs/superpowers/plans/2026-05-10-wsl-vpnfix-phase-c-audit-and-release.md`.
3. Implementation proceeds against the plan: three documentation PRs in the order given (threat model → audit → master-spec rebase), then the `v0.2.0` tag.
