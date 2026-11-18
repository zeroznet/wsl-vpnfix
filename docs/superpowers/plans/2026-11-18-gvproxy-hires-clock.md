<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 5.5) -->

# gvproxy High-Resolution Clock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a `gvproxy-windowsgui.exe` built from pinned upstream source with a QueryPerformanceCounter stack clock, which removes the RACK spurious retransmits on the bridge.

**Architecture:** One local patch to containers/gvisor-tap-vsock lives in `build/patches/`. A new `gvproxy-builder` Dockerfile stage clones the pinned tag, checks the commit, applies the patch, unit-tests the clock math and cross-compiles. The `fetcher` stage keeps only gvforwarder.

**Tech Stack:** Go 1.26 (apk `go=1.26.11-r0`), gVisor `tcpip.Clock`, `golang.org/x/sys/windows` (vendored upstream), podman, POSIX sh.

**Spec:** `docs/superpowers/specs/2026-11-18-gvproxy-hires-clock-design.md`

## Global Constraints

- Upstream: `containers/gvisor-tap-vsock` tag `v0.9.0`, commit `6dbb5d4fa0033356c72a201346f8350594229868`.
- gvproxy build: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0`, `-trimpath -mod=vendor`, ldflags `-s -w -buildid= -H=windowsgui -X github.com/containers/gvisor-tap-vsock/pkg/types.gitVersion=${GVTV_TAG}-wsl-vpnfix`.
- No new network fetch except `git clone` of the pinned tag. Upstream vendors its modules, so the build stays offline after the clone.
- Commits: single line, lowercase. Push, PR, tag and upstream posts only on Robert's explicit go.
- Attribution header on every new file (`//` in Go, `#` in sh/yaml/patch preamble, `<!-- -->` in markdown).

## Review Focus

1. QPC counter after long uptime: `counter * 1e9` overflows int64 after ~15 min at 10 MHz. Pinned by `TestQPCDurationNoOverflow` (Task 1).
2. Renovate bumps `tag:` but not `commit:`. The build must fail on commit mismatch, not build an unverified tree. Pinned by Task 2 Step 4.
3. The patch stops applying on a new upstream tag. `git apply --check` must fail the build loudly. Pinned by Task 2 Step 5.
4. Reproducibility: two builds from one commit must give the same gvproxy.exe SHA-256. Pinned by Task 2 Step 7.
5. Upgrade over v0.2.x: the orchestrator restages `C:\Users\Public\.wsl-vpnfix\wsl-gvproxy.exe` when the SHA differs (`stageGvproxyExe`). Checked at smoke, Task 4.

---

### Task 1: Upstream patch (QPC stack clock)

Work in a scratch clone: `git clone --branch v0.9.0 https://github.com/containers/gvisor-tap-vsock`. The deliverable is `build/patches/gvproxy-hires-clock.patch` in this repo.

**Files (in the upstream clone):**
- Create: `pkg/virtualnetwork/qpc.go` (no build tag)
- Create: `pkg/virtualnetwork/qpc_test.go`
- Create: `pkg/virtualnetwork/clock_windows.go`
- Create: `pkg/virtualnetwork/clock_other.go` (`//go:build !windows`)
- Modify: `pkg/virtualnetwork/virtualnetwork.go` (`createStack`, `stack.New` options)

**Interfaces:**
- Produces: `func qpcDuration(counter, freq int64) time.Duration`, `func newStackClock() tcpip.Clock` (nil means gVisor's default clock).

- [ ] **Step 1: Write the failing tests in `qpc_test.go`**

```go
func TestQPCDuration(t *testing.T) {
	cases := []struct{ counter, freq int64; want time.Duration }{
		{1000, 1000, time.Second},
		{1, 3, 333_333_333 * time.Nanosecond},
		{0, 10_000_000, 0},
	}
	// assert qpcDuration(c.counter, c.freq) == c.want for each
}

func TestQPCDurationNoOverflow(t *testing.T) {
	// 30 days at 24 MHz and 1000 s at 10 MHz: exact, no int64 wrap
	// qpcDuration(24_000_000*86400*30, 24_000_000) == 30*24*time.Hour
	// qpcDuration(10_000_000*1000, 10_000_000) == 1000*time.Second
}
```

- [ ] **Step 2: Run, expect FAIL** (`undefined: qpcDuration`)

Run: `go test ./pkg/virtualnetwork -run TestQPC`

- [ ] **Step 3: Implement `qpcDuration` in `qpc.go`**

Split into whole seconds plus remainder: `(counter/freq)*1e9 + (counter%freq)*1e9/freq`.

- [ ] **Step 4: Run, expect PASS**

Run: `go test ./pkg/virtualnetwork -run TestQPC`

- [ ] **Step 5: Implement the Windows clock and wire it**

`clock_windows.go`: `type qpcClock struct { tcpip.Clock; freq, base int64 }`. It embeds `tcpip.NewStdClock()` for `Now` and `AfterFunc` and overrides `NowMonotonic() tcpip.MonotonicTime` as `tcpip.MonotonicTime{}.Add(qpcDuration(qpc()-c.base, c.freq))`. QueryPerformanceFrequency and QueryPerformanceCounter come from `windows.NewLazySystemDLL("kernel32.dll")` (vendored `x/sys/windows` has no wrapper). `newStackClock()` returns nil and logs a warning via logrus if either call fails or freq <= 0. `clock_other.go`: `newStackClock()` returns nil. `createStack`: add `Clock: newStackClock()` to `stack.Options`.

- [ ] **Step 6: Verify both targets**

Run: `go vet ./pkg/virtualnetwork && GOOS=windows go vet ./pkg/virtualnetwork ./cmd/gvproxy && GOOS=windows GOARCH=amd64 go build -mod=vendor -o /dev/null ./cmd/gvproxy && go test ./pkg/virtualnetwork`
Expected: no output from vet, build OK, tests PASS.

- [ ] **Step 7: Export the patch and commit**

`git diff --no-color` of the five files into `build/patches/gvproxy-hires-clock.patch`, preceded by the attribution header and a 3-line `#` preamble (what, why, upstream status). Then `git apply --check` the patch on a fresh v0.9.0 clone.

```bash
git add build/patches/gvproxy-hires-clock.patch
git commit -m "build: gvproxy patch giving gVisor a QueryPerformanceCounter clock on Windows (RACK spurious retransmits)"
```

### Task 2: Build pipeline

**Files:**
- Modify: `build/Dockerfile.rootfs` (new stage between `builder` and `fetcher`, fetcher loses the exe, final COPY source)
- Modify: `build/upstream-pins.yaml` (add `commit:`, remove the `gvproxy-windowsgui.exe` sha block, update header procedure)
- Modify: `build/pack.sh:67-111` (parse `commit:`, drop `GVTV_GVPROXY_EXE_SHA256`, pass `GVTV_COMMIT`)
- Modify: `.github/workflows/ci.yml:87-108` (rename the step to "Verify upstream pins (fetcher + gvproxy-builder stages)", same parsing, builds both targets)

**Interfaces:**
- Consumes: `build/patches/gvproxy-hires-clock.patch` (Task 1).
- Produces: build args `GVTV_TAG`, `GVTV_COMMIT`, `GVTV_GVFORWARDER_SHA256`; stage `gvproxy-builder` output `/out/gvproxy-windowsgui.exe`.

- [ ] **Step 1: Add stage `gvproxy-builder`**

`FROM builder AS gvproxy-builder` (reuses the pinned apk go and git). Args `GVTV_TAG`, `GVTV_COMMIT`. `COPY build/patches/gvproxy-hires-clock.patch /patches/`. Then, in one `RUN set -eu`:
1. `git clone --depth 1 --branch "$GVTV_TAG"`
2. `test "$(git rev-parse HEAD)" = "$GVTV_COMMIT"`, otherwise die with `gvisor-tap-vsock ${GVTV_TAG} is $(git rev-parse HEAD), pinned ${GVTV_COMMIT}`
3. `git apply --check` then `git apply`
4. `GOFLAGS=-mod=vendor go test ./pkg/virtualnetwork -run TestQPC`
5. the Windows build per Global Constraints into `/out/gvproxy-windowsgui.exe`

Change the final stage to `COPY --from=gvproxy-builder /out/gvproxy-windowsgui.exe /etc/wsl-vpnfix/wsl-gvproxy.exe`. Remove the exe download and `GVTV_GVPROXY_EXE_SHA256` from `fetcher`. Update the header's stage list.

- [ ] **Step 2: Pins file and pack.sh**

`upstream-pins.yaml`: `commit: 6dbb5d4fa0033356c72a201346f8350594229868` under `tag:`. Bump procedure: step 3 becomes "replace tag, commit and the gvforwarder sha256; rebase the patch if `git apply --check` fails". pack.sh: `GVTV_COMMIT=$(awk '/^  commit:/ { print $2; exit }')`, validated as 40 hex chars (same `case` style as the sha checks), die `could not parse gvisor-tap-vsock commit from ${PINS_FILE}`.

- [ ] **Step 3: CI step** builds `--target fetcher` and `--target gvproxy-builder` with the parsed args.

- [ ] **Step 4: Prove the commit guard**

Run the stage build with `GVTV_COMMIT=0000000000000000000000000000000000000000`.
Expected: build fails with the `pinned 0000…` message.

- [ ] **Step 5: Prove the patch guard**

Run the stage build with `GVTV_TAG=v0.8.9` and that tag's commit (`git ls-remote` it).
Expected: fails at `git apply --check` (if the patch happens to apply there, record that and skip).

- [ ] **Step 6: Full build**

Run: `./build/pack.sh` (needs a host with working podman; sentinel's is broken, see TODO, so use CI or another host).
Expected: tarball in `out/`. `tar -xOf <tarball> etc/wsl-vpnfix/checksums` lists the new exe SHA.

- [ ] **Step 7: Reproducibility**

Run `pack.sh` twice from the same commit.
Expected: identical tarball SHA-256.

- [ ] **Step 8: Commit**

```bash
git add build/Dockerfile.rootfs build/upstream-pins.yaml build/pack.sh .github/workflows/ci.yml
git commit -m "build: gvproxy.exe built from pinned gvisor-tap-vsock source + hires-clock patch, gvforwarder stays sha256-pinned release binary"
```

### Task 3: Docs, audit, TODO, memory (same PR, per #11 sweep)

**Files:**
- Modify: `README.md:39,190,197` ("every external binary pinned by SHA-256" and "pulled verbatim from upstream releases" become: gvforwarder pinned by SHA-256; gvproxy.exe built from the pinned tag + commit with one patch, linked)
- Modify: `docs/THREAT-MODEL.md:81-83` (supply chain: source pin + patch for gvproxy)
- Modify: `docs/SECURITY-AUDIT.md` (new F-019: shipped gvproxy.exe is a locally patched build; mitigation commit pin + `git apply --check` + reproducible build; status: until upstream fix)
- Modify: `CLAUDE.md` (conventions "Pin third-party binaries", upstream-dependencies paragraph, layout lines for `build/`, add `build/patches/`)
- Modify: `TODO.md` (retransmit item: replace with "Drop `gvproxy-hires-clock.patch` once a gvisor-tap-vsock release carries an equivalent fix"; periodic upstream check: commit + patch-rebase steps)
- Modify: memory `project_gvisor_tap_vsock_v088.md` (gvproxy now locally built and patched)

- [ ] **Step 1: Edit the files above**
- [ ] **Step 2: Sweep:** `grep -rn "gvproxy-windowsgui.exe\|GVPROXY_EXE\|pulled verbatim\|every external binary" --include="*.md" --include="*.sh" --include="*.yml" --include="*.yaml" --include="*.json" --include="Dockerfile*" .` excluding frozen `docs/superpowers/` and `docs/smoke-*`. Expected: only intentional hits.
- [ ] **Step 3: Commit:** `git commit -m "docs: gvproxy.exe is now built from pinned source with the hires-clock patch (README, threat model, F-019, CLAUDE.md, todo)"`

### Task 4: Upstream drafts, PR, release, smoke

- [ ] **Step 1: Draft** the upstream issue (cause, three-row evidence table, one-line fix direction) and the PR description for containers/gvisor-tap-vsock. Run both through slopbeth. Show them to Robert; post only on "pošli".
- [ ] **Step 2:** On Robert's go: push the branch, open the PR, CI green, merge per `project_pr_merge_dance_with_codex.md`.
- [ ] **Step 3:** Once all v0.3.0 bundle items are merged and Robert gives the go: tag `v0.3.0`, release pipeline green, assets present.
- [ ] **Step 4: Smoke on sonic** (one smoke for the whole release):

```sh
nstat -n ; curl -so /dev/null -w '%{http_code} %{size_download} %{speed_download}\n' https://proof.ovh.net/files/10Mb.dat ; nstat TcpInSegs TcpExtTCPDSACKOldSent ; curl -sI https://1.1.1.1 | head -1
```

Expected: 200 with full size, `TcpExtTCPDSACKOldSent` near 0 (was 42137), `HTTP/2 301`. The orchestrator log shows gvproxy `v0.9.0-wsl-vpnfix` and that the exe was restaged.

- [ ] **Step 5:** Write `docs/smoke-<date>.md`, set the CLAUDE.md status to v0.3.0 and delete the done TODO lines, in one docs PR.
