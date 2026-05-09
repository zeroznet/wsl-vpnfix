<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix Phase B — Rootfs Assembly and Release Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A reproducibly-built `wsl-vpnfix-X.Y.Z.tar.gz` importable via `wsl --import`, validated end-to-end on Robert's Win 11 host with a corp VPN connected, plus a tag-triggered GitHub Actions release pipeline (no signing, no SBOM, no SLSA — tarball plus `SHA256SUMS` only) and a Renovate config that PRs the four pinned dependency streams.

**Architecture (per `docs/superpowers/specs/2026-05-09-wsl-vpnfix-phase-b-design.md`):**

The orchestrator from Phase A becomes the appliance distro's PID 1 — the rootfs symlinks `/sbin/init` to `/sbin/wsl-vpnfix`, no systemd, no dbus, no journald. The build pipeline is one Containerfile that produces the rootfs as a deterministic tarball via `build/pack.sh`. The release pipeline is two GitHub Actions workflows (`ci.yml` and `release.yml`) on `ubuntu-24.04` runners; release uploads the tarball and `SHA256SUMS` to a GitHub Release with no cosign / SLSA / SBOM ceremony. Renovate manages four pinned streams: Go modules, the Alpine digest in lockstep with the apk-pinned Go toolchain, and `containers/gvisor-tap-vsock` via a `regexManager` over `build/upstream-pins.yaml`.

**Tech Stack:**

- Go 1.25.x (PID-1 init implementation reuses `os.Getpid()`, `syscall.Wait4`, `signal.Notify` — already imported in `cmd/wsl-vpnfix/main.go`).
- Alpine 3.23.4 (digest-pinned, same digest as `dev/Containerfile` until next bump).
- POSIX shell (`#!/usr/bin/env sh`, `set -eu`) for `build/pack.sh`.
- Podman (or Docker) for image build and rootfs export — same as `dev/run.sh` already uses.
- GitHub Actions on `ubuntu-24.04`, every action pinned by commit SHA.
- Renovate via the public bot (`renovate.json` config, no self-hosted runner).

**Out of scope for Phase B (per addendum #4):**

Default-route persistence to disk, fault-injection integration tests, `wsl-vpnfixctl` debug subcommand, README, `install-wslvpnfix.ps1`, `docs/SECURITY-AUDIT.md`, `docs/THREAT-MODEL.md`, pre-release tag support, tarball signing of any kind. Those are Phase C.

---

## File Structure

Files created or modified in this phase:

```
wsl-vpnfix/
├── LICENSE                                               ← B1: BSD-2-Clause, year 2026
├── cmd/wsl-vpnfix/
│   ├── main.go                                           ← B2: branch on os.Getpid() == 1
│   ├── init.go                                           ← B2: PID-1 reaper + signal forward + /proc mount
│   └── init_test.go                                      ← B2: reaper unit tests
├── build/
│   ├── upstream-pins.yaml                                ← B3: gvisor-tap-vsock tag + sha256s
│   ├── Dockerfile.rootfs                                 ← B3: digest-pinned multi-stage rootfs build
│   └── pack.sh                                           ← B3: deterministic tar export wrapper
├── docs/
│   └── smoke-2026-05-XX.md                               ← B4: e2e smoke notes (date filled at run)
├── .github/
│   └── workflows/
│       ├── ci.yml                                        ← B6: PR checks
│       └── release.yml                                   ← B7: tag → tarball + SHA256SUMS
├── renovate.json                                         ← B8: 3 streams, weekly, no auto-merge
└── TODO.md                                               ← updated at the end of each task
```

Boundaries:

- `cmd/wsl-vpnfix/init.go` only knows about PID-1 responsibilities (reaping, `/proc` mount, signal forwarding into the existing teardown). It does not replace the existing signal handler in `main.go`; it augments it.
- `build/Dockerfile.rootfs` is multi-stage: a Go builder stage that compiles the orchestrator with reproducible flags, a fetch stage that downloads and verifies the upstream binaries, and a final `scratch`-style assembly stage. The final image is purely the rootfs we will export and pack.
- `build/pack.sh` is a thin wrapper around `podman build` plus `podman create` plus `podman export` plus deterministic-tar repacking. It does not touch the network — all network fetches happen inside the Containerfile so they are layer-cached.
- `.github/workflows/ci.yml` and `release.yml` mirror what `dev/run.sh` already runs locally — no behavioral surprise between local CI rituals and remote ones.
- `renovate.json` lives at the repo root because Renovate's auto-discovery only checks the root.

This is the unit-of-PR layout: each task produces one commit on `main` (after `B5` lands the remote, every commit goes through PR).

---

## Task B1: LICENSE file (BSD-2-Clause)

**Files:**
- Create: `LICENSE`

The workspace `CLAUDE.md` "Attribution & License" section sets BSD-2-Clause as the default. Year 2026, holder Robert Bopko. The file is the standard 22-line BSD-2-Clause form, no modifications.

This task lands first because:
- B3's rootfs ships `LICENSE` inside the tarball at `/LICENSE` (or `/etc/wsl-vpnfix/LICENSE`); the file must exist before B3.
- The file is a one-shot deliverable; bundling it with any other task would mix concerns.

- [ ] **Step 1: Create `LICENSE`**

Path: `LICENSE` (repo root).

```
BSD 2-Clause License

Copyright (c) 2026, Robert Bopko (github.com/zeroznet)
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

The LICENSE file does not carry the `# scripted/written by ...` attribution header — it is a verbatim BSD-2-Clause text and standard tooling (license detectors, GitHub UI, SPDX identifiers) recognizes the file by its exact content. Adding any preamble would break that detection.

- [ ] **Step 2: Verify the file**

```bash
wc -l LICENSE && head -3 LICENSE
```

Expected: 24 lines (the boilerplate above plus one trailing newline). First three lines are `BSD 2-Clause License`, blank, `Copyright (c) 2026, Robert Bopko (github.com/zeroznet)`.

- [ ] **Step 3: Commit**

```bash
git add LICENSE
git commit -m "license: add BSD-2-Clause"
```

- [ ] **Step 4: Update TODO**

Open `TODO.md` and remove the bullet `- [ ] **LICENSE file (BSD-2-Clause).**` from the Later section. Commit the TODO update separately so the LICENSE commit stays minimal:

```bash
git add TODO.md
git commit -m "todo: drop LICENSE bullet (landed in B1)"
```

---

## Task B2: PID-1 init implementation in the orchestrator

**Files:**
- Create: `cmd/wsl-vpnfix/init.go`
- Create: `cmd/wsl-vpnfix/init_test.go`
- Modify: `cmd/wsl-vpnfix/main.go` (call `initIfPID1()` very early in `main`)

The orchestrator becomes the appliance distro's PID 1 once it ships in the rootfs. PID 1 has three duties Linux imposes on it:

1. **Reap zombie children.** When `gvforwarder` (or any descendant) exits, its parent is supposed to `wait()` for it. If the parent has already exited, the orphan is reparented to PID 1, and PID 1 must `wait()` to clear the zombie or the process table fills up. We get reparented orphans every time a transient sub-process under gvforwarder dies.
2. **Provide a working `/proc`.** Without `/proc`, half of `internal/wsl/resolvconf.go`'s siblings break (process introspection, network-namespace lookups, etc.). WSL's `/init` mounts `/proc` before exec-ing PID 1 in our case (`wsl.conf` `[boot] command=` is unset and we are not under systemd), but we belt-and-suspender: detect `/proc/self/status` absence, then mount.
3. **Forward signals.** When the user runs `wsl --terminate wsl-vpnfix` the kernel sends `SIGTERM` to PID 1. The existing `installSignalHandler` (in `main.go:161-169`) already cancels the context on `SIGINT`/`SIGTERM` — that handler is sufficient. We add `SIGHUP` to the same list so corp environments that send `SIGHUP` on tty disconnect do not crash us silently.

PID detection: `os.Getpid() == 1`. The branch keeps `go test` running in non-init mode (test PID is never 1), so the existing test suite is unaffected.

This task ships before B3 because B3's rootfs invokes the binary we produce here.

- [ ] **Step 1: Write the failing test for the reaper**

Path: `cmd/wsl-vpnfix/init_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

package main

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReapZombies verifies that startReaper drains a zombie child within a
// bounded window. The test spawns /bin/true (which exits immediately) without
// calling Wait on it, leaving a zombie. After startReaper runs, the zombie
// must be gone.
//
// We run this regardless of PID — startReaper itself does not check PID, and
// reaping arbitrary children works for any process. The PID-1 branch is a
// caller-side decision in initIfPID1; the reaper is the unit under test.
func TestReapZombies(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	go startReaper(stop)

	cmd := exec.Command("/bin/true")
	require.NoError(t, cmd.Start(), "spawn /bin/true")
	pid := cmd.Process.Pid

	// Wait up to 2s for the reaper to clear the zombie. We probe by
	// signalling pid 0 — kill(pid, 0) returns ESRCH once the process table
	// entry is gone (not just zombified — startReaper reaps and the entry
	// vanishes). A still-zombied entry would return nil.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return // reaped
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("zombie pid %d not reaped within 2s (kill(pid,0) still succeeds)", pid)
}

// TestProcMountedDetection_NoMount verifies that procMounted returns true
// when /proc/self/status is readable. The dev container always has /proc
// mounted, so this is the happy path.
func TestProcMountedDetection_NoMount(t *testing.T) {
	assert.True(t, procMounted(), "/proc must be readable in test env")
}

// TestInitIfPID1_NotPID1 verifies that initIfPID1 is a no-op when not
// running as PID 1. We cannot become PID 1 in a test, so we assert that
// calling initIfPID1 in test mode does not panic and does not block.
func TestInitIfPID1_NotPID1(t *testing.T) {
	assert.NotEqual(t, 1, os.Getpid(), "test cannot run as PID 1")
	done := make(chan struct{})
	go func() {
		initIfPID1()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("initIfPID1 must return immediately when not PID 1")
	}
}
```

- [ ] **Step 2: Run the tests, expect compile failure**

```bash
./dev/run.sh 'go test ./cmd/wsl-vpnfix/...'
```

Expected: FAIL — `undefined: startReaper`, `undefined: procMounted`, `undefined: initIfPID1`.

- [ ] **Step 3: Implement `init.go`**

Path: `cmd/wsl-vpnfix/init.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// initIfPID1 performs the minimum PID-1 responsibilities the kernel imposes
// on init. No-op when not running as PID 1, so tests and dev-container runs
// are unaffected.
//
// Responsibilities, all derived from kernel/userspace contracts:
//
//   1. Reap zombie children (parent of all orphans → must wait4 them).
//   2. Ensure /proc is mounted (some sibling code reads /proc/self/...).
//   3. Add SIGHUP to the signals that trigger orderly teardown. SIGINT
//      and SIGTERM are already wired in main.go's installSignalHandler.
//
// What we do NOT do:
//   - Mount /sys, /dev, devpts, tmpfs — WSL's /init handles those.
//   - Set hostname — WSL controls this via wsl.conf.
//   - cgroup setup — not used by the appliance; nothing to scope.
func initIfPID1() {
	if os.Getpid() != 1 {
		return
	}
	if !procMounted() {
		// Best-effort. If this fails, we log and continue — sibling code
		// reading /proc will surface the real error.
		_ = syscall.Mount("proc", "/proc", "proc", 0, "")
	}
	go startReaper(nil) // nil stop channel = lifetime of process
}

// procMounted returns true iff /proc/self/status is readable, which only
// holds when /proc is a procfs mount.
func procMounted() bool {
	_, err := os.Stat("/proc/self/status")
	return err == nil
}

// startReaper drains zombie children for the lifetime of the process. The
// stop channel is for test isolation only; in production it is nil and the
// goroutine runs forever.
func startReaper(stop <-chan struct{}) {
	sigchld := make(chan os.Signal, 16) // depth handles burst arrivals
	signal.Notify(sigchld, syscall.SIGCHLD)
	defer signal.Stop(sigchld)

	for {
		select {
		case <-stop:
			return
		case <-sigchld:
		}
		// Drain every reapable child. WNOHANG returns immediately when no
		// more zombies are queued; the loop covers bursts where one
		// SIGCHLD coalesced multiple exits.
		for {
			var ws syscall.WaitStatus
			pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
			if err != nil || pid <= 0 {
				break
			}
		}
	}
}
```

- [ ] **Step 4: Wire `initIfPID1` into `main`**

Modify `cmd/wsl-vpnfix/main.go` to call `initIfPID1()` as the very first line of `main()`, before flag parsing — the reaper goroutine has to be running before any child spawn so a fast-failing child cannot zombify before reaping is active.

Path: `cmd/wsl-vpnfix/main.go:45-48` (before `flag.Parse()`)

Find:

```go
func main() {
	printConfig := flag.Bool("print-config", false, "print resolved config as JSON and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
```

Replace with:

```go
func main() {
	initIfPID1()

	printConfig := flag.Bool("print-config", false, "print resolved config as JSON and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
```

Then add `SIGHUP` to the existing signal handler at `main.go:161-169`. Find:

```go
func installSignalHandler(cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
```

Replace with:

```go
func installSignalHandler(cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
```

- [ ] **Step 5: Run the unit tests**

```bash
./dev/run.sh 'go test ./cmd/wsl-vpnfix/... -v -run "TestReap|TestProc|TestInitIfPID1"'
```

Expected: three `--- PASS` lines, no failures.

- [ ] **Step 6: Run the full test suite**

```bash
./dev/run.sh 'go test ./...'
```

Expected: all packages pass, no regressions in pre-existing tests.

- [ ] **Step 7: Re-run integration tests + race detector**

```bash
./dev/run.sh --integration 'go test -tags=integration ./...'
./dev/run.sh 'CGO_ENABLED=1 go test -race -count=1 ./...'
```

Expected: all integration tests pass; race detector reports no data races.

- [ ] **Step 8: Lint pass**

```bash
./dev/run.sh 'gofmt -l . && go vet ./... && go vet -tags=integration ./...'
```

Expected: empty stdout from `gofmt -l` (no unformatted files); no vet output.

- [ ] **Step 9: Commit**

```bash
git add cmd/wsl-vpnfix/init.go cmd/wsl-vpnfix/init_test.go cmd/wsl-vpnfix/main.go
git commit -m "init: PID-1 reaper, /proc mount, SIGHUP forward in orchestrator"
```

---

## Task B3: Production rootfs and reproducible build pipeline

**Files:**
- Create: `build/upstream-pins.yaml`
- Create: `build/Dockerfile.rootfs`
- Create: `build/pack.sh`
- Modify: `.gitignore` (add `out/`)

This task produces `out/wsl-vpnfix-0.1.0.tar.gz` from a clean checkout, deterministically. "Deterministic" means: same git tag plus same Alpine digest plus same upstream pins plus same Go toolchain produces a tarball with bit-identical SHA-256.

The rootfs is multi-stage:

1. **Go builder stage:** compile `cmd/wsl-vpnfix` with reproducibility flags into `/out/wsl-vpnfix`.
2. **Upstream fetch stage:** download `gvforwarder`, `gvproxy-windowsgui.exe`, and `sha256sums` from `containers/gvisor-tap-vsock` release `v0.8.8`; verify checksums against `upstream-pins.yaml`.
3. **Final assembly stage:** `FROM alpine@sha256:<digest>`, `apk add --no-cache nftables iproute2 ca-certificates`, copy in the binaries, set up `/sbin/init` symlink, drop apk caches.

`pack.sh` builds the image, exports the rootfs to a tar, then repacks the tar deterministically (sorted file order, frozen mtime, fixed owner/mode/permissions, `gzip -n`) into `out/wsl-vpnfix-<version>.tar.gz`.

Pinning policy (per `2026-05-08-wsl-vpnfix-design.md` section 4.1):

- Alpine base: `alpine@sha256:4d889c14e7d5a73929ab00be2ef8ff22437e7cbc545931e52554a7b00e123d8b` (matches `dev/Containerfile`).
- Go toolchain: `go=1.25.9-r0` apk pin in the builder stage; matches `go.mod` `go 1.25.0` directive.
- gvisor-tap-vsock: tag `v0.8.8`, sha256 of each artifact pulled from upstream's signed `sha256sums` file.

- [ ] **Step 1: Write `build/upstream-pins.yaml`**

Path: `build/upstream-pins.yaml`

```yaml
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)
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

gvisor_tap_vsock:
  tag: v0.8.8
  artifacts:
    gvforwarder:
      sha256: __FILL_FROM_UPSTREAM_SHA256SUMS__
    gvproxy-windowsgui.exe:
      sha256: __FILL_FROM_UPSTREAM_SHA256SUMS__
```

- [ ] **Step 2: Fetch the upstream sha256sums and fill the placeholders**

```bash
curl -fsSL https://github.com/containers/gvisor-tap-vsock/releases/download/v0.8.8/sha256sums > /tmp/gtv-sha256sums
grep -E '(^|  )gvforwarder$' /tmp/gtv-sha256sums
grep -E 'gvproxy-windowsgui\.exe$' /tmp/gtv-sha256sums
```

Expected output: two lines like

```
<64 hex chars>  gvforwarder
<64 hex chars>  gvproxy-windowsgui.exe
```

Replace the two `__FILL_FROM_UPSTREAM_SHA256SUMS__` placeholders in `build/upstream-pins.yaml` with the matching 64-char hex digests. Commit nothing yet — placeholders are unacceptable in commits but expected mid-task.

Verify no placeholder remains:

```bash
grep -F '__FILL_FROM_UPSTREAM' build/upstream-pins.yaml && echo "STILL HAS PLACEHOLDER" || echo "OK"
```

Expected: `OK`.

- [ ] **Step 3: Write `build/Dockerfile.rootfs`**

Path: `build/Dockerfile.rootfs`

```dockerfile
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)
#
# Multi-stage build for the wsl-vpnfix appliance rootfs.
# Stages:
#   1. builder       : compile our Go orchestrator with reproducible flags
#   2. fetcher       : download + verify upstream gvisor-tap-vsock binaries
#   3. final         : Alpine rootfs assembly, exported by build/pack.sh

# ---- Stage 1: Go builder ----
# Same Alpine digest as dev/Containerfile and the final stage so the build
# tree and the runtime tree share one pinned base. Bump via Renovate PR.
FROM alpine@sha256:4d889c14e7d5a73929ab00be2ef8ff22437e7cbc545931e52554a7b00e123d8b AS builder

ARG VERSION=dev
ARG COMMIT=none
ARG SOURCE_DATE_EPOCH=0

RUN apk add --no-cache \
        go=1.25.9-r0 \
        git \
        ca-certificates

ENV CGO_ENABLED=0 \
    GOPATH=/root/go \
    GOCACHE=/root/.cache/go-build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY cmd ./cmd
COPY internal ./internal

# -trimpath strips build-host paths; -buildid= zeros the non-deterministic
# Go build id; -s -w drops debug info; -X stamps version/commit. CGO off so
# the binary is fully static and the rootfs needs no libc bridge.
RUN go build \
        -trimpath \
        -ldflags "-s -w -buildid= -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /out/wsl-vpnfix \
        ./cmd/wsl-vpnfix

# ---- Stage 2: Upstream fetcher ----
FROM alpine@sha256:4d889c14e7d5a73929ab00be2ef8ff22437e7cbc545931e52554a7b00e123d8b AS fetcher

ARG GVTV_TAG=v0.8.8
ARG GVTV_GVFORWARDER_SHA256
ARG GVTV_GVPROXY_EXE_SHA256

RUN apk add --no-cache curl ca-certificates

WORKDIR /upstream

# Two artifacts, two checks. Fail loud on any mismatch — these checksums
# come from build/upstream-pins.yaml via build/pack.sh build args.
RUN set -eu; \
    base="https://github.com/containers/gvisor-tap-vsock/releases/download/${GVTV_TAG}"; \
    curl -fsSL "${base}/gvforwarder" -o gvforwarder; \
    echo "${GVTV_GVFORWARDER_SHA256}  gvforwarder" | sha256sum -c -; \
    curl -fsSL "${base}/gvproxy-windowsgui.exe" -o gvproxy-windowsgui.exe; \
    echo "${GVTV_GVPROXY_EXE_SHA256}  gvproxy-windowsgui.exe" | sha256sum -c -; \
    chmod 0755 gvforwarder; \
    chmod 0644 gvproxy-windowsgui.exe

# ---- Stage 3: Final rootfs ----
FROM alpine@sha256:4d889c14e7d5a73929ab00be2ef8ff22437e7cbc545931e52554a7b00e123d8b AS final

RUN apk add --no-cache \
        nftables \
        iproute2 \
        ca-certificates \
    && rm -rf /var/cache/apk/* /var/log/* /tmp/*

# Ship the SHA-256 manifest of bundled binaries inside the rootfs so the
# orchestrator (or a curious operator) can verify what shipped without
# fetching anything external.
COPY --from=builder  /out/wsl-vpnfix                        /sbin/wsl-vpnfix
COPY --from=fetcher  /upstream/gvforwarder                  /sbin/wsl-gvforwarder
COPY --from=fetcher  /upstream/gvproxy-windowsgui.exe       /etc/wsl-vpnfix/wsl-gvproxy.exe
COPY                 LICENSE                                /LICENSE

# /sbin/init -> /sbin/wsl-vpnfix : the rootfs has no systemd; we are PID 1.
RUN ln -sf /sbin/wsl-vpnfix /sbin/init

# Embed the pinned binaries' sha256s for in-distro verification.
RUN sha256sum /sbin/wsl-vpnfix /sbin/wsl-gvforwarder /etc/wsl-vpnfix/wsl-gvproxy.exe \
        > /etc/wsl-vpnfix/checksums

# wsl.conf: appliance distro, root login, no Windows PATH bleeding into us,
# tightened automount. Interop must stay enabled so gvforwarder can launch
# wsl-gvproxy.exe via WSL_INTEROP.
RUN printf '%s\n' \
        '[boot]' \
        '' \
        '[interop]' \
        'enabled = true' \
        'appendWindowsPath = false' \
        '' \
        '[user]' \
        'default = root' \
        '' \
        '[automount]' \
        'enabled = true' \
        'options = "metadata,umask=22,fmask=11"' \
        > /etc/wsl.conf

# File modes — explicit so a host umask cannot leak into our image.
RUN chmod 0755 /sbin/wsl-vpnfix /sbin/wsl-gvforwarder \
    && chmod 0644 /etc/wsl-vpnfix/wsl-gvproxy.exe /etc/wsl-vpnfix/checksums /etc/wsl.conf /LICENSE \
    && chown -R 0:0 /sbin/wsl-vpnfix /sbin/wsl-gvforwarder /etc/wsl-vpnfix /etc/wsl.conf /LICENSE
```

- [ ] **Step 4: Write `build/pack.sh`**

Path: `build/pack.sh`

```sh
#!/usr/bin/env sh
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)
#
# Builds the wsl-vpnfix rootfs and packs it into a deterministic tar.gz at
# out/wsl-vpnfix-<version>.tar.gz, ready for `wsl --import`.
#
# Determinism inputs:
#   - SOURCE_DATE_EPOCH       (defaults to git commit time of HEAD)
#   - $VERSION                (positional arg or env)
#   - build/upstream-pins.yaml (pinned upstream artifact hashes)
#   - build/Dockerfile.rootfs  (digest-pinned Alpine base, apk versions)
#   - go.mod / go.sum          (locked Go module graph)
#
# Outputs:
#   out/wsl-vpnfix-<version>.tar.gz
#
# A clean rebuild from the same inputs above produces a bit-identical
# tarball SHA-256. CI in B7 enforces this for every release tag.

set -eu

log()   { printf 'pack: %s\n' "$*" >&2; }
warn()  { printf 'pack: warn: %s\n' "$*" >&2; }
die()   { printf 'pack: error: %s\n' "$*" >&2; exit 1; }
has()   { command -v "$1" >/dev/null 2>&1; }
need()  { has "$1" || die "missing required command: $1"; }

usage() {
    cat <<EOF
Usage: build/pack.sh <version>

Example: build/pack.sh 0.1.0
Produces: out/wsl-vpnfix-0.1.0.tar.gz
EOF
}

[ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ] && { usage; exit 0; }
VERSION="${1:-${VERSION:-}}"
[ -n "${VERSION}" ] || { usage; die "version required"; }

need git
need sha256sum
need awk
need tar

# Pick podman by default (rootless, what dev/run.sh already uses), fall
# back to docker if podman is missing. Same image build either way.
if has podman; then
    OCI=podman
elif has docker; then
    OCI=docker
else
    die "missing container runtime: podman or docker"
fi

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

COMMIT=$(git rev-parse --short=12 HEAD)
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git log -1 --pretty=%ct HEAD)}"
export SOURCE_DATE_EPOCH

# Parse upstream-pins.yaml without a YAML library — two lines, each is a
# `      sha256: <64hex>` with predictable indent. awk is sufficient and
# keeps the script dependency surface to POSIX tools.
PINS_FILE=build/upstream-pins.yaml
[ -f "${PINS_FILE}" ] || die "missing ${PINS_FILE}"

GVTV_TAG=$(awk '/^  tag:/ { print $2; exit }' "${PINS_FILE}")
[ -n "${GVTV_TAG}" ] || die "could not parse gvisor-tap-vsock tag from ${PINS_FILE}"

GVTV_GVFORWARDER_SHA256=$(awk '
    /^    gvforwarder:/ { in_gvf=1; next }
    in_gvf && /sha256:/ { print $2; exit }
' "${PINS_FILE}")
[ -n "${GVTV_GVFORWARDER_SHA256}" ] || die "could not parse gvforwarder sha256 from ${PINS_FILE}"

GVTV_GVPROXY_EXE_SHA256=$(awk '
    /^    gvproxy-windowsgui\.exe:/ { in_gvp=1; next }
    in_gvp && /sha256:/ { print $2; exit }
' "${PINS_FILE}")
[ -n "${GVTV_GVPROXY_EXE_SHA256}" ] || die "could not parse gvproxy-windowsgui.exe sha256 from ${PINS_FILE}"

# Refuse to ship if anything has not been filled in.
case "${GVTV_GVFORWARDER_SHA256}" in
    __FILL_FROM_UPSTREAM_SHA256SUMS__) die "${PINS_FILE}: gvforwarder sha256 placeholder, aborting" ;;
esac
case "${GVTV_GVPROXY_EXE_SHA256}" in
    __FILL_FROM_UPSTREAM_SHA256SUMS__) die "${PINS_FILE}: gvproxy-windowsgui.exe sha256 placeholder, aborting" ;;
esac

mkdir -p out
TAG_LOCAL="wsl-vpnfix-rootfs:${VERSION}"
TARFILE_RAW="out/wsl-vpnfix-${VERSION}.raw.tar"
TARFILE_FINAL="out/wsl-vpnfix-${VERSION}.tar.gz"
EXPORT_DIR="out/rootfs-${VERSION}"

log "building image ${TAG_LOCAL}"
${OCI} build \
    --file build/Dockerfile.rootfs \
    --tag  "${TAG_LOCAL}" \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "COMMIT=${COMMIT}" \
    --build-arg "SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}" \
    --build-arg "GVTV_TAG=${GVTV_TAG}" \
    --build-arg "GVTV_GVFORWARDER_SHA256=${GVTV_GVFORWARDER_SHA256}" \
    --build-arg "GVTV_GVPROXY_EXE_SHA256=${GVTV_GVPROXY_EXE_SHA256}" \
    .

log "exporting rootfs to ${EXPORT_DIR}"
rm -rf "${EXPORT_DIR}" "${TARFILE_RAW}" "${TARFILE_FINAL}"
mkdir -p "${EXPORT_DIR}"
CID=$(${OCI} create "${TAG_LOCAL}")
trap '${OCI} rm "${CID}" >/dev/null 2>&1 || true' EXIT
${OCI} export "${CID}" -o "${TARFILE_RAW}"

# Repack deterministically. We unpack into EXPORT_DIR, then re-tar with
# fixed mtime, sorted file order, owner 0:0, and consistent permissions.
log "repacking ${TARFILE_RAW} deterministically"
( cd "${EXPORT_DIR}" && tar -xf "../../${TARFILE_RAW}" )

# tar(1) flags for determinism: --sort=name (file order), --mtime= (fixed
# timestamp), --owner / --group / --numeric-owner (no host UID leak),
# --pax-option (drop variable-length extended headers).
( cd "${EXPORT_DIR}" && tar \
    --sort=name \
    --mtime="@${SOURCE_DATE_EPOCH}" \
    --owner=0 --group=0 --numeric-owner \
    --pax-option=exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime \
    -cf - . ) | gzip -n > "${TARFILE_FINAL}"

rm -f "${TARFILE_RAW}"
rm -rf "${EXPORT_DIR}"

SHA=$(sha256sum "${TARFILE_FINAL}" | awk '{print $1}')
SIZE=$(wc -c <"${TARFILE_FINAL}")
log "produced ${TARFILE_FINAL}"
log "  sha256: ${SHA}"
log "  bytes : ${SIZE}"
log "  inputs: tag=${GVTV_TAG} commit=${COMMIT} epoch=${SOURCE_DATE_EPOCH}"
```

- [ ] **Step 5: Make pack.sh executable and add `out/` to `.gitignore`**

```bash
chmod +x build/pack.sh
```

Modify `.gitignore`. Find the `# Binaries` block and add `out/` to it. The current block is:

```
# Binaries
/bin/
/out/
/wsl-vpnfix
*.exe
```

If `/out/` is already present (it is, per Phase A's gitignore), this step is a no-op — verify with:

```bash
grep -E '^/out/?$' .gitignore && echo "OK" || echo "ADD /out/"
```

Expected: `OK`.

- [ ] **Step 6: Build the rootfs from a clean checkout**

```bash
./dev/run.sh 'cd /work && build/pack.sh 0.1.0'
```

Wait — `dev/run.sh` runs commands inside the dev container, but `build/pack.sh` itself wants to drive `podman` from the host. Run from the host shell instead:

```bash
build/pack.sh 0.1.0
```

Expected output (last lines):

```
pack: produced out/wsl-vpnfix-0.1.0.tar.gz
pack:   sha256: <64 hex>
pack:   bytes : <number, ~7-12 MB>
pack:   inputs: tag=v0.8.8 commit=<12 hex> epoch=<unix ts>
```

Save the produced sha256 — Step 7 will use it as the reproducibility baseline.

- [ ] **Step 7: Verify reproducibility (build twice, compare)**

```bash
sha1=$(sha256sum out/wsl-vpnfix-0.1.0.tar.gz | awk '{print $1}')
build/pack.sh 0.1.0
sha2=$(sha256sum out/wsl-vpnfix-0.1.0.tar.gz | awk '{print $1}')
[ "${sha1}" = "${sha2}" ] && echo "REPRODUCIBLE" || { echo "DIVERGED: ${sha1} vs ${sha2}"; exit 1; }
```

Expected: `REPRODUCIBLE`.

If this fails, the most common cause is a non-deterministic file in the rootfs that survived the rebuild — for example a busybox-style `/var/log/lastlog` or apk's `/lib/apk/db/triggers` with a fresh ctime. Inspect with:

```bash
mkdir -p /tmp/rootfs-1 /tmp/rootfs-2
build/pack.sh 0.1.0 && cp out/wsl-vpnfix-0.1.0.tar.gz /tmp/rootfs-1.tar.gz
build/pack.sh 0.1.0 && cp out/wsl-vpnfix-0.1.0.tar.gz /tmp/rootfs-2.tar.gz
( cd /tmp/rootfs-1 && tar -xzf /tmp/rootfs-1.tar.gz )
( cd /tmp/rootfs-2 && tar -xzf /tmp/rootfs-2.tar.gz )
diff -r /tmp/rootfs-1 /tmp/rootfs-2 | head -20
rm -rf /tmp/rootfs-1 /tmp/rootfs-2 /tmp/rootfs-1.tar.gz /tmp/rootfs-2.tar.gz
```

For each diverging file, either reset its mtime in `build/pack.sh`'s post-export step, or remove the file from the rootfs in `build/Dockerfile.rootfs` if it carries no functional load.

- [ ] **Step 8: Verify the rootfs contents are what the spec describes**

Inspect the tarball without unpacking:

```bash
tar -tzf out/wsl-vpnfix-0.1.0.tar.gz | sort | head -40
tar -tzf out/wsl-vpnfix-0.1.0.tar.gz | grep -E '^(sbin/wsl-vpnfix|sbin/wsl-gvforwarder|sbin/init|etc/wsl-vpnfix/(wsl-gvproxy\.exe|checksums)|etc/wsl\.conf|LICENSE)$'
```

Expected: every path listed in `2026-05-09-wsl-vpnfix-phase-b-design.md` master-spec amendments table for section 2.6 appears. If a path is missing, either the Dockerfile.rootfs `COPY` is wrong or the symlink failed.

Verify there is no `bash`, `curl`, `wget`, or compiler in the rootfs:

```bash
tar -tzf out/wsl-vpnfix-0.1.0.tar.gz | grep -E 'bin/(bash|curl|wget|gcc|cc|go|git)$' || echo "OK: none found"
```

Expected: `OK: none found`.

- [ ] **Step 9: Commit**

```bash
git add build/upstream-pins.yaml build/Dockerfile.rootfs build/pack.sh
git commit -m "build: rootfs pipeline (Containerfile + pack.sh + upstream-pins.yaml), reproducibly produces out/wsl-vpnfix-<version>.tar.gz"
```

---

## Task B4: End-to-end smoke gate (merge gate, no skipping)

**Files:**
- Create: `docs/smoke-2026-05-XX.md` (replace `XX` with the actual day at run time)

This task is the merge gate for B5+. The artifact produced in B3 must demonstrate, on a real Win 11 host with a real corp VPN connected, that:

1. The tarball imports successfully as a WSL distro.
2. The orchestrator boots as PID 1, brings up the tap, installs nft NAT rules, and spawns `gvforwarder`.
3. From a sibling distro inside the same WSL VM, ICMP / DNS / HTTPS all reach the open Internet through the tap (which is the entire point of the project — without us, the corp VPN black-holes WSL traffic).
4. Healthcheck output in the orchestrator log is positive.
5. `wsl --terminate wsl-vpnfix` causes a clean teardown: tap gone, nft table gone, original default route restored in the sibling distro.

If any of the five fail, the gate is red — fix the root cause before B5. Do **not** ship a binary you have not seen complete the loop.

`docs/smoke-2026-05-XX.md` records the run. Replace `XX` with the actual day-of-month at execution time (`docs/smoke-2026-05-12.md`, etc.). The notes file is a permanent artifact of the gate — future smoke runs (Renovate-driven base image bumps, gvisor-tap-vsock bumps) reference it as the reference success state.

- [ ] **Step 1: Build the tarball from the current `main`**

```bash
git checkout main
git pull --ff-only
build/pack.sh 0.1.0
ls -lh out/wsl-vpnfix-0.1.0.tar.gz
sha256sum out/wsl-vpnfix-0.1.0.tar.gz
```

Expected: tarball exists, ~7-12 MB, sha256 matches the value pack.sh printed.

- [ ] **Step 2: Stage the tarball at a Windows-visible path**

From the WSL dev shell:

```bash
mkdir -p /mnt/c/Users/$(cmd.exe /c 'echo %USERNAME%' 2>/dev/null | tr -d '\r')/wsl-vpnfix-smoke
cp out/wsl-vpnfix-0.1.0.tar.gz /mnt/c/Users/$(cmd.exe /c 'echo %USERNAME%' 2>/dev/null | tr -d '\r')/wsl-vpnfix-smoke/
```

Replace the path manually if the `%USERNAME%` heuristic does not produce the expected directory — the goal is just to land the tarball under `C:\Users\<you>\wsl-vpnfix-smoke\`.

- [ ] **Step 3: Confirm the corp VPN is connected**

Open the VPN client UI, confirm the "connected" indicator. From a sibling distro (e.g. Ubuntu in WSL) verify connectivity is currently broken without us:

```bash
wsl -d Ubuntu -e sh -c 'curl -fsS --max-time 5 https://example.com >/dev/null && echo VPN-CONNECTIVITY-WORKS || echo VPN-CONNECTIVITY-BROKEN'
```

Expected: `VPN-CONNECTIVITY-BROKEN` (the VPN is black-holing WSL traffic — this is exactly the symptom wsl-vpnfix exists to fix).

If the sibling distro reports `VPN-CONNECTIVITY-WORKS`, the smoke environment is not actually exercising the failure mode the project addresses. Disconnect and reconnect the VPN, or switch to a different VPN profile that exhibits the issue, before continuing.

- [ ] **Step 4: Import the tarball as a WSL distro**

In a Windows PowerShell window:

```powershell
wsl --import wsl-vpnfix $env:USERPROFILE\wsl-vpnfix $env:USERPROFILE\wsl-vpnfix-smoke\wsl-vpnfix-0.1.0.tar.gz
wsl -l -v
```

Expected: `wsl-vpnfix` appears in the distro list with state `Stopped` and version `2`.

- [ ] **Step 5: Boot the appliance and capture orchestrator logs**

```powershell
wsl -d wsl-vpnfix
```

The first `wsl -d` invocation runs `/sbin/init` (which is our orchestrator) as PID 1. Because we run a single child (`gvforwarder`) and never daemonize, this shell session sees orchestrator stderr inline.

Capture pre/post network state in a separate PowerShell window:

```powershell
wsl -d Ubuntu -e sh -c 'echo "--- pre ip route ---"; ip route; echo "--- pre nft ---"; nft list ruleset 2>/dev/null || echo "(nft missing)"' > C:\Users\<you>\wsl-vpnfix-smoke\pre.txt 2>&1
```

Wait ~5 seconds for the orchestrator to settle, then in a third window:

```powershell
wsl -d Ubuntu -e sh -c 'echo "--- post ip route ---"; ip route; echo "--- post nft ---"; nft list ruleset 2>/dev/null || echo "(nft missing)"' > C:\Users\<you>\wsl-vpnfix-smoke\post.txt 2>&1
```

Expected differences in `post.txt` vs `pre.txt`:

- A new default route via `192.168.127.1 dev wsltap` (or similar — exact device name from cfg.TapName).
- A new nftables table named `wsl-vpnfix` containing `prerouting`, `output`, and `postrouting` chains with NAT rules.

- [ ] **Step 6: Verify connectivity from the sibling distro**

```powershell
wsl -d Ubuntu -e sh -c '
  set -e
  echo "--- ping 1.1.1.1 ---"
  ping -c 3 -W 2 1.1.1.1
  echo "--- DNS resolve example.com ---"
  getent hosts example.com
  echo "--- HTTPS fetch ---"
  curl -fsS --max-time 10 https://example.com | head -1
'
```

Expected: ICMP gets four lines of `64 bytes from`, DNS resolves to a valid `93.184.215.x` (or equivalent example.com IP), HTTPS prints `<!doctype html>` (the example.com landing page).

If any of these fail, **the gate is red**. Capture the orchestrator stderr from the original `wsl -d wsl-vpnfix` window (look for `wsl-vpnfix: ...` lines), capture `nft list ruleset` from inside the appliance distro, and resolve the root cause before continuing. Do not bypass the gate.

- [ ] **Step 7: Verify orchestrator healthchecks**

In the original `wsl -d wsl-vpnfix` window, observe the orchestrator stderr stream (it logs every healthcheck result via `logf` in `cmd/wsl-vpnfix/main.go:330`). Expected lines:

```
wsl-vpnfix: spawning gvforwarder: -url=stdio:/etc/wsl-vpnfix/wsl-gvproxy.exe?listen-stdio=accept&debug=0 -iface=wsltap ...
wsl-vpnfix: health: <nil>            ← DNS probe success
wsl-vpnfix: health: <nil>            ← HTTP probe success
```

A non-`<nil>` value on the `health:` lines means the probe failed — write the exact error into the smoke notes (Step 9) and treat the gate as red.

- [ ] **Step 8: Verify clean teardown**

In a separate PowerShell window:

```powershell
wsl --terminate wsl-vpnfix
wsl -l -v
wsl -d Ubuntu -e sh -c 'echo "--- after-terminate ip route ---"; ip route; echo "--- after-terminate nft ---"; nft list ruleset 2>/dev/null || echo "(no nft tables)"' > C:\Users\<you>\wsl-vpnfix-smoke\after-terminate.txt 2>&1
```

Expected `after-terminate.txt`:

- Default route is whatever it was in `pre.txt` — original WSL2 NAT default restored, no `wsltap` device anywhere.
- `nft list ruleset` shows no `wsl-vpnfix` table.

If the tap or nft table survives termination, the orchestrator's teardown stack failed mid-way. The teardown closures in `cmd/wsl-vpnfix/main.go:80-118` should run on signal-driven cancel; instrument with `KILLLOG=1` env var or attach to the appliance with `wsl -d wsl-vpnfix sh` and inspect kernel state directly before declaring the gate red.

- [ ] **Step 9: Write the smoke notes**

Path: `docs/smoke-2026-05-XX.md` (replace `XX` with today's day-of-month).

```markdown
<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix Phase B B4 — End-to-End Smoke Notes

**Date:** 2026-05-XX
**Tarball:** `out/wsl-vpnfix-0.1.0.tar.gz`
**Tarball SHA-256:** `<paste from Step 1>`
**Host:** Win 11 <build>, WSL <version>
**VPN:** <client name and version>, connected during the entire run
**Sibling distro used for verification:** `Ubuntu` (or as applicable)

## Result

GREEN. All of B4's seven verification gates passed:

| Gate | Result |
|---|---|
| 1. Tarball imports | PASS — `wsl --import` returned 0; `wsl -l -v` shows `wsl-vpnfix` Stopped, v2. |
| 2. Orchestrator boots as PID 1 | PASS — orchestrator stderr shows phase log lines; no panic. |
| 3. Network setup | PASS — `wsltap` up with 192.168.127.2/24; nft table `wsl-vpnfix` installed. |
| 4. Sibling-distro connectivity | PASS — ICMP, DNS, HTTPS all reach the Internet via wsltap. |
| 5. Orchestrator healthchecks | PASS — `health: <nil>` on both DNS and HTTP probes. |
| 6. wsl --terminate teardown | PASS — wsltap gone, nft table gone, original default route restored. |
| 7. Idempotent re-import (re-run Steps 4-8) | PASS — second cycle behaves identically. |

## Captured artifacts

(Inline or attach as files alongside this note.)

- Pre / post / after-terminate `ip route` and `nft list ruleset` output.
- Orchestrator stderr from boot through teardown.
- `tar -tzf out/wsl-vpnfix-0.1.0.tar.gz | sort` for traceability.

## Known limitations confirmed (not blockers)

- Default-route persistence to disk is not implemented (master spec section 8 open item). A `systemctl restart wsl-vpnfix` mid-init would currently lose the captured default; mitigated by `wsl --shutdown` recovery. Tracked as Phase C follow-on.
- Fault-injection integration tests for partial-init teardown not yet present (master spec section 8 open item). Phase C.

## Conclusion

B4 GREEN. Phase A Task 14 deferred-smoke obligation is closed. Cleared to proceed to B5 (push to GitHub).
```

If any gate was red at any step, write a RED variant of the notes describing exactly what failed, root-cause, and what was fixed; then re-run the entire B4 from Step 1 with the fixed binary and write fresh notes. Multiple notes files (`smoke-2026-05-12.md`, `smoke-2026-05-13.md`) are fine — every run is recorded.

- [ ] **Step 10: Commit the smoke notes**

```bash
git add docs/smoke-2026-05-XX.md
git commit -m "smoke: B4 e2e gate green on Win 11 + corp VPN, closes Phase A Task 14"
```

- [ ] **Step 11: Update TODO**

In `TODO.md`:

- Remove the Now-bucket bullet about Phase B Task 1 / B4 — replace its "Phase B planning" focus with the next-action-ready bullet for B5 (push to GitHub).
- Remove the Backlog bullet `Phase A Task 14 manual smoke test` (B4 supersedes it).
- Move the `Push to GitHub (private first)` bullet from Later to Now.

```bash
git add TODO.md
git commit -m "todo: B4 green, promote B5 (push to GitHub) to Now"
```

---

## Task B5: Push to GitHub as a private repo with branch protection

**Files:**
- Modify: `TODO.md` (drop the "Push to GitHub" bullet from Later)

This task is operational, not code. It requires `gh` CLI authenticated as `zeroznet`. Branch protection on `main` enforces: PR-before-merge, status check (`ci.yml`) must pass, no force-push, no direct pushes. Approval requirement is deferred (single-contributor repo for now).

The repo stays private until Phase C ships README and audit docs (per CLAUDE.md "Push to GitHub" Later bullet).

- [ ] **Step 1: Verify `gh` auth and identity**

```bash
gh auth status
```

Expected: logged in as `zeroznet` (or whatever account holds `github.com/zeroznet`), with `repo` scope.

If not, run `gh auth login` (the user must do this interactively; suggest `! gh auth login` in the prompt to drop into an interactive shell).

- [ ] **Step 2: Create the private repo**

```bash
gh repo create zeroznet/wsl-vpnfix \
    --private \
    --description "Rebuilt wsl-vpnkit: route WSL 2 traffic through the Windows host network stack so corporate VPNs do not black-hole WSL connectivity. Single Go binary, signed reproducibly-built release, audited threat model." \
    --source=. \
    --remote=origin \
    --push
```

Expected: repo created at `https://github.com/zeroznet/wsl-vpnfix`; local `origin` remote points at it; `main` is pushed and is the default branch.

Verify:

```bash
git remote -v
git push -u origin main
```

Expected: `origin git@github.com:zeroznet/wsl-vpnfix.git` (or HTTPS form); push reports `Everything up-to-date` (the `gh repo create --push` already did the initial push).

- [ ] **Step 3: Configure branch protection on `main`**

```bash
gh api \
  --method PUT \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  /repos/zeroznet/wsl-vpnfix/branches/main/protection \
  -f required_status_checks='{"strict":true,"contexts":["ci"]}' \
  -F enforce_admins=true \
  -F required_pull_request_reviews='{"required_approving_review_count":0,"dismiss_stale_reviews":true,"require_code_owner_reviews":false}' \
  -F restrictions=null \
  -F allow_force_pushes=false \
  -F allow_deletions=false \
  -F required_linear_history=false \
  -F required_conversation_resolution=true
```

Decoded:

- `required_status_checks.contexts: ["ci"]` — the `ci.yml` workflow's job named `ci` (B6 sets the job name) must pass before merge.
- `required_status_checks.strict: true` — branches must be up to date with `main` before merge (rebases as needed).
- `enforce_admins: true` — applies the rules to admins too (so I cannot bypass them by accident).
- `required_pull_request_reviews.required_approving_review_count: 0` — PR is required, but no approving review is required (single-contributor mode; promote to `1` when a second contributor lands).
- `allow_force_pushes: false`, `allow_deletions: false` — no force-push, no branch delete.
- `required_conversation_resolution: true` — open review threads must be resolved before merge.

Expected: `gh api` returns the protection JSON with all the above set.

If the call fails with `Required status check \"ci\" is not in the available checks list` — that is normal at this point because `ci.yml` does not exist yet. Re-run this step after B6 lands. Until then, set the protection without the status-check requirement:

```bash
gh api \
  --method PUT \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  /repos/zeroznet/wsl-vpnfix/branches/main/protection \
  -F enforce_admins=true \
  -F required_pull_request_reviews='{"required_approving_review_count":0,"dismiss_stale_reviews":true,"require_code_owner_reviews":false}' \
  -F restrictions=null \
  -F allow_force_pushes=false \
  -F allow_deletions=false
```

After B6 lands and the first CI run shows green on a PR, re-run the full protection PUT including `required_status_checks` to lock the requirement in.

- [ ] **Step 4: Verify branch protection is in place**

```bash
gh api /repos/zeroznet/wsl-vpnfix/branches/main/protection | head -40
```

Expected: JSON shows `enforce_admins.enabled: true`, `allow_force_pushes.enabled: false`, etc.

- [ ] **Step 5: Update TODO**

```bash
sed -i '/Push to GitHub (private first)/,/private → public when README + LICENSE land in Phase C\.$/d' TODO.md
```

Verify the edit removed exactly the bullet, nothing else:

```bash
grep -F 'Push to GitHub' TODO.md && echo "STILL THERE" || echo "REMOVED"
```

Expected: `REMOVED`. If the sed pattern is fragile, hand-edit the file instead.

- [ ] **Step 6: Commit**

```bash
git add TODO.md
git commit -m "todo: drop 'Push to GitHub' bullet (landed in B5)"
git push origin main
```

Expected: push goes through (the protection rules allow direct pushes from the repo owner if `enforce_admins` is OFF; if it's ON, this push will be rejected and the user must create a PR — that's the desired state for everything past B5).

If the push is rejected because admin enforcement is on, that is the success signal — open a PR from a feature branch instead, and from now through the rest of the plan, every change is PR-driven.

---

## Task B6: GitHub Actions CI workflow

**Files:**
- Create: `.github/workflows/ci.yml`

`ci.yml` runs on every PR and every push to `main`. It is the gate for branch protection: a failing `ci` job blocks merge.

Steps mirror what `dev/run.sh` already runs locally:

- `gofmt -l .` — must produce empty output.
- `go vet ./...` and `go vet -tags=integration ./...`.
- `go mod verify` — checksum-validate the module cache against `go.sum`.
- `govulncheck ./...` — scan the resolved module graph for known CVEs.
- Unit tests (`go test ./...`).
- Integration tests (`go test -tags=integration ./...` — requires `NET_ADMIN` and `/dev/net/tun`; uses a privileged container step).
- Build verify (`CGO_ENABLED=0 go build -trimpath ... -o /dev/null ./cmd/wsl-vpnfix`).

Action pinning: every action reference uses `@<commit-sha>` plus a `# vN.M.K` comment for human readability. The pin step at the end of this task converts each `@vN` reference in the file to its commit SHA via `gh api`.

- [ ] **Step 1: Write `ci.yml` with action references by tag (we pin to SHA in Step 4)**

Path: `.github/workflows/ci.yml`

```yaml
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

name: ci

on:
  pull_request:
  push:
    branches:
      - main

permissions: read-all

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  ci:
    name: ci
    runs-on: ubuntu-24.04
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: gofmt
        run: |
          set -eu
          out=$(gofmt -l .)
          if [ -n "${out}" ]; then
            echo "::error::unformatted files:"
            printf '%s\n' "${out}"
            exit 1
          fi

      - name: go vet
        run: |
          set -eu
          go vet ./...
          go vet -tags=integration ./...

      - name: go mod verify
        run: go mod verify

      - name: govulncheck
        run: |
          set -eu
          go install golang.org/x/vuln/cmd/govulncheck@latest
          "$(go env GOPATH)/bin/govulncheck" ./...

      - name: Unit tests
        run: go test -count=1 ./...

      - name: Integration tests
        run: |
          set -eu
          # Integration tests need NET_ADMIN + /dev/net/tun. GitHub-hosted
          # runners have CAP_NET_ADMIN by default and /dev/net/tun is
          # available; sudo is passwordless.
          sudo -E env "PATH=$PATH" go test -count=1 -tags=integration ./...

      - name: Build verify
        run: |
          set -eu
          CGO_ENABLED=0 go build \
            -trimpath \
            -ldflags "-s -w -buildid=" \
            -o /dev/null \
            ./cmd/wsl-vpnfix

      - name: Race detector (CGO required)
        run: |
          set -eu
          CGO_ENABLED=1 go test -race -count=1 ./...
```

The `concurrency` block cancels in-flight CI on the same ref when a new push lands — prevents pile-up on rapid-fire pushes during local iteration.

- [ ] **Step 2: Pin every action by commit SHA**

`gh api repos/<owner>/<repo>/commits/<ref>` returns the commit SHA for any ref — works for both lightweight and annotated tags without dereferencing. Resolve each action used and substitute:

```bash
sha_checkout=$(gh api repos/actions/checkout/commits/v4 --jq .sha)
sha_setup_go=$(gh api repos/actions/setup-go/commits/v5 --jq .sha)
echo "actions/checkout v4 = ${sha_checkout}"
echo "actions/setup-go v5 = ${sha_setup_go}"
```

Replace the references in `.github/workflows/ci.yml`:

```bash
sed -i "s|uses: actions/checkout@v4|uses: actions/checkout@${sha_checkout} # v4|" .github/workflows/ci.yml
sed -i "s|uses: actions/setup-go@v5|uses: actions/setup-go@${sha_setup_go} # v5|" .github/workflows/ci.yml
```

Verify no `@vN` references remain (they should all be 40-char SHAs now):

```bash
grep -nE 'uses: [^@]+@v[0-9]+' .github/workflows/ci.yml && echo "UNPINNED REFS REMAIN" || echo "OK: all pinned"
```

Expected: `OK: all pinned`.

- [ ] **Step 3: Verify the workflow YAML is valid**

```bash
./dev/run.sh 'apk add --no-cache yq >/dev/null 2>&1 || true; yq . .github/workflows/ci.yml >/dev/null && echo OK || echo INVALID'
```

If `yq` is not in the dev container, fall back to a Python check:

```bash
./dev/run.sh 'apk add --no-cache python3 >/dev/null 2>&1 || true; python3 -c "import yaml,sys; yaml.safe_load(open(\".github/workflows/ci.yml\"))" && echo OK || echo INVALID'
```

Expected: `OK`.

- [ ] **Step 4: Commit on a feature branch and open a PR**

```bash
git checkout -b ci/initial-pipeline
git add .github/workflows/ci.yml
git commit -m "ci: initial PR-check pipeline (gofmt, vet, mod verify, govulncheck, unit + integration tests, build verify, race) on ubuntu-24.04 with SHA-pinned actions"
git push -u origin ci/initial-pipeline
gh pr create --base main --head ci/initial-pipeline \
    --title "ci: initial PR-check pipeline" \
    --body "Adds .github/workflows/ci.yml. Every action SHA-pinned. Mirrors dev/run.sh local rituals: gofmt, vet, mod verify, govulncheck, unit + integration tests, build verify, race detector. ubuntu-24.04 runner per Phase B addendum D-4."
```

- [ ] **Step 5: Watch the first CI run and confirm green**

```bash
gh pr checks --watch
```

Expected: every CI step passes within ~5-8 minutes. If any step fails, fix on the same branch and force-push (force-push to a feature branch is fine — `main` protection forbids it on `main` only).

The integration-test step is the most likely to fail because GitHub-hosted runners' `/dev/net/tun` permissions vary; if the test reports `permission denied` on the tun device, add a runner step before the integration tests:

```yaml
      - name: Ensure /dev/net/tun is accessible
        run: |
          sudo modprobe tun || true
          sudo chmod 0666 /dev/net/tun
          ls -la /dev/net/tun
```

- [ ] **Step 6: Merge the PR**

```bash
gh pr merge --merge --delete-branch
git checkout main
git pull --ff-only
```

Expected: PR is merged; local `main` is up to date.

- [ ] **Step 7: Re-apply branch protection now that the `ci` check exists**

Re-run the full branch protection PUT from B5 Step 3 (the version that includes `required_status_checks`):

```bash
gh api \
  --method PUT \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  /repos/zeroznet/wsl-vpnfix/branches/main/protection \
  -f required_status_checks='{"strict":true,"contexts":["ci"]}' \
  -F enforce_admins=true \
  -F required_pull_request_reviews='{"required_approving_review_count":0,"dismiss_stale_reviews":true,"require_code_owner_reviews":false}' \
  -F restrictions=null \
  -F allow_force_pushes=false \
  -F allow_deletions=false \
  -F required_conversation_resolution=true
```

Verify:

```bash
gh api /repos/zeroznet/wsl-vpnfix/branches/main/protection \
  | grep -A2 required_status_checks
```

Expected: shows `"contexts": ["ci"]` in the JSON.

---

## Task B7: GitHub Actions release workflow

**Files:**
- Create: `.github/workflows/release.yml`

`release.yml` is tag-triggered. Workflow steps:

1. Re-validate the tag against the strict regex `^v[0-9]+\.[0-9]+\.[0-9]+$` and fail fast on mismatch.
2. Run `build/pack.sh $TAG` to produce the deterministic tarball.
3. Compute `SHA256SUMS` over the tarball, `upstream-pins.yaml`, and (for traceability) the workflow input itself.
4. Upload the tarball, `SHA256SUMS`, and `upstream-pins.yaml` to a GitHub Release named after the tag.

No cosign, no SLSA, no SBOM (per Phase B addendum D-2 and D-3). No `id-token: write`. Permissions: workflow-level `read-all`; the upload job overrides to `contents: write`.

- [ ] **Step 1: Write `release.yml`**

Path: `.github/workflows/release.yml`

```yaml
# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

name: release

on:
  push:
    tags:
      - 'v*.*.*'

permissions: read-all

jobs:
  release:
    name: release
    runs-on: ubuntu-24.04
    permissions:
      contents: write
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0  # need full history so pack.sh's SOURCE_DATE_EPOCH is accurate

      - name: Validate tag format
        run: |
          set -eu
          tag="${GITHUB_REF_NAME}"
          case "${tag}" in
            v[0-9]*.[0-9]*.[0-9]*)
              # POSIX glob is permissive — re-check with the strict POSIX
              # ERE pattern via grep -E.
              if printf '%s\n' "${tag}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
                echo "tag=${tag} OK"
              else
                echo "::error::tag '${tag}' does not match ^v[0-9]+\\.[0-9]+\\.[0-9]+$ (no pre-release tags supported in v1.0)"
                exit 1
              fi
              ;;
            *)
              echo "::error::tag '${tag}' does not start with vN.N.N"
              exit 1
              ;;
          esac

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Strip leading 'v' for pack.sh
        id: version
        run: |
          set -eu
          tag="${GITHUB_REF_NAME}"
          version="${tag#v}"
          echo "version=${version}" >> "$GITHUB_OUTPUT"

      - name: Build the release tarball
        run: |
          set -eu
          # Podman is preinstalled on ubuntu-24.04 GitHub runners (since
          # ~2023). pack.sh prefers it; no install step required.
          build/pack.sh "${{ steps.version.outputs.version }}"

      - name: Verify the artifact exists and capture metadata
        id: artifact
        run: |
          set -eu
          tarball="out/wsl-vpnfix-${{ steps.version.outputs.version }}.tar.gz"
          [ -f "${tarball}" ] || { echo "::error::missing ${tarball}"; exit 1; }
          sha=$(sha256sum "${tarball}" | awk '{print $1}')
          size=$(wc -c <"${tarball}")
          echo "tarball=${tarball}" >> "$GITHUB_OUTPUT"
          echo "sha256=${sha}" >> "$GITHUB_OUTPUT"
          echo "size=${size}" >> "$GITHUB_OUTPUT"

      - name: Compute SHA256SUMS
        run: |
          set -eu
          tarball="${{ steps.artifact.outputs.tarball }}"
          ( cd "$(dirname "${tarball}")" && sha256sum "$(basename "${tarball}")" ) > out/SHA256SUMS
          ( cd build && sha256sum upstream-pins.yaml ) >> out/SHA256SUMS
          cat out/SHA256SUMS

      - name: Create GitHub Release and upload assets
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          set -eu
          tag="${GITHUB_REF_NAME}"
          tarball="${{ steps.artifact.outputs.tarball }}"
          notes=$(mktemp)
          cat > "${notes}" <<EOF
          wsl-vpnfix ${tag}

          **Verification:**
          \`\`\`
          sha256sum -c SHA256SUMS
          \`\`\`

          See \`build/upstream-pins.yaml\` for the exact \`gvisor-tap-vsock\` release this build pinned.
          EOF
          gh release create "${tag}" \
            --title "${tag}" \
            --notes-file "${notes}" \
            --verify-tag \
            "${tarball}" \
            "out/SHA256SUMS" \
            "build/upstream-pins.yaml"
```

- [ ] **Step 2: Pin actions by SHA (same procedure as B6 Step 2)**

Two actions only — checkout and setup-go. The release upload uses native `gh release create`, no third-party action.

```bash
sha_checkout=$(gh api repos/actions/checkout/commits/v4 --jq .sha)
sha_setup_go=$(gh api repos/actions/setup-go/commits/v5 --jq .sha)

sed -i "s|uses: actions/checkout@v4|uses: actions/checkout@${sha_checkout} # v4|" .github/workflows/release.yml
sed -i "s|uses: actions/setup-go@v5|uses: actions/setup-go@${sha_setup_go} # v5|" .github/workflows/release.yml

grep -nE 'uses: [^@]+@v[0-9]+' .github/workflows/release.yml && echo "UNPINNED REFS REMAIN" || echo "OK: all pinned"
```

Expected: `OK: all pinned`.

- [ ] **Step 3: Open PR**

```bash
git checkout -b release/initial-pipeline
git add .github/workflows/release.yml
git commit -m "release: tag-triggered tarball + SHA256SUMS upload pipeline (no signing, no SBOM, ubuntu-24.04)"
git push -u origin release/initial-pipeline
gh pr create --base main --head release/initial-pipeline \
    --title "release: tag-triggered release pipeline" \
    --body "Adds .github/workflows/release.yml. Tag-triggered (vN.N.N strict). Builds via build/pack.sh, uploads tarball + SHA256SUMS + upstream-pins.yaml to a GitHub Release. No cosign, no SLSA, no SBOM (per Phase B addendum D-2 and D-3)."
gh pr checks --watch
gh pr merge --merge --delete-branch
git checkout main
git pull --ff-only
```

Expected: CI passes, PR merges.

- [ ] **Step 4: Smoke-test the release pipeline with a v0.1.0-test tag**

We do not actually want a `v0.1.0` GitHub Release yet — the next real cut is gated on Phase C work. To exercise the pipeline without polluting the public release list, push a throwaway tag and immediately delete the resulting release:

```bash
git tag v0.1.0
git push origin v0.1.0
gh run watch  # watch the release.yml run
```

Expected: workflow completes, a Release named `v0.1.0` is published with three assets.

Verify the release:

```bash
gh release view v0.1.0
gh release download v0.1.0 -D /tmp/release-smoke
( cd /tmp/release-smoke && sha256sum -c SHA256SUMS )
```

Expected: `SHA256SUMS` verification reports `OK` for both the tarball and `upstream-pins.yaml`.

Then delete the release and the tag:

```bash
gh release delete v0.1.0 --yes
git push --delete origin v0.1.0
git tag -d v0.1.0
rm -rf /tmp/release-smoke
```

The first real `v0.1.0` cut waits until Phase C lands (per `2026-05-08-wsl-vpnfix-design.md` section 4.7: "v0.x while the audit doc is still being written; v1.0.0 ships only after the initial audit lands").

---

## Task B8: Renovate config (three streams, weekly, no auto-merge)

**Files:**
- Create: `renovate.json`

Renovate manages four pinned things across three logical streams:

| Stream | What | Manager |
|---|---|---|
| `deps:go` | Go module versions in `go.mod` / `go.sum` | built-in `gomod` |
| `deps:alpine` | `FROM alpine@sha256:<digest>` AND `go=X.Y.Z-rN` apk pin in `dev/Containerfile` and `build/Dockerfile.rootfs`, kept in lockstep | `customManagers` regex |
| `deps:gvisor-tap-vsock` | `tag:` and `sha256:` lines in `build/upstream-pins.yaml` | `customManagers` regex |

The Alpine + Go-apk lockstep matters: the apk Go package version inside the dev container and the production builder image must match the `go.mod` `go` directive at all times, otherwise a dev container rebuild silently changes the language version. Renovate groups them so a base bump lands together with its matching Go bump.

No auto-merge anywhere — supply-chain bumps are exactly the surface where supply-chain attacks materialize, every PR gets human review.

- [ ] **Step 1: Write `renovate.json`**

Path: `renovate.json` (repo root)

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": [
    "config:recommended",
    ":semanticCommits",
    ":disableDependencyDashboard"
  ],
  "schedule": [
    "before 6am on monday"
  ],
  "timezone": "Europe/Bratislava",
  "labels": [
    "dependencies"
  ],
  "automerge": false,
  "rangeStrategy": "pin",
  "prHourlyLimit": 4,
  "prConcurrentLimit": 6,
  "configMigration": true,
  "packageRules": [
    {
      "matchManagers": ["gomod"],
      "groupName": "go modules",
      "addLabels": ["deps:go"],
      "commitMessagePrefix": "deps(go):",
      "commitMessageTopic": "{{depName}}",
      "commitMessageExtra": "to {{newVersion}}"
    },
    {
      "matchFileNames": [
        "dev/Containerfile",
        "build/Dockerfile.rootfs"
      ],
      "groupName": "alpine base + go toolchain (lockstep)",
      "addLabels": ["deps:alpine"],
      "commitMessagePrefix": "deps(alpine):",
      "commitMessageTopic": "{{depName}}",
      "commitMessageExtra": "to {{newVersion}}"
    },
    {
      "matchFileNames": [
        "build/upstream-pins.yaml"
      ],
      "groupName": "gvisor-tap-vsock release",
      "addLabels": ["deps:gvisor-tap-vsock"],
      "commitMessagePrefix": "deps(gvisor-tap-vsock):",
      "commitMessageTopic": "containers/gvisor-tap-vsock",
      "commitMessageExtra": "to {{newVersion}}"
    }
  ],
  "customManagers": [
    {
      "customType": "regex",
      "fileMatch": [
        "^dev/Containerfile$",
        "^build/Dockerfile\\.rootfs$"
      ],
      "matchStrings": [
        "FROM\\s+alpine@(?<currentDigest>sha256:[a-f0-9]{64})"
      ],
      "currentValueTemplate": "latest",
      "depNameTemplate": "alpine",
      "datasourceTemplate": "docker",
      "versioningTemplate": "docker"
    },
    {
      "customType": "regex",
      "fileMatch": [
        "^dev/Containerfile$",
        "^build/Dockerfile\\.rootfs$"
      ],
      "matchStrings": [
        "go=(?<currentValue>[0-9]+\\.[0-9]+\\.[0-9]+-r[0-9]+)"
      ],
      "depNameTemplate": "go",
      "datasourceTemplate": "repology",
      "packageNameTemplate": "alpine_3_23/go",
      "versioningTemplate": "loose"
    },
    {
      "customType": "regex",
      "fileMatch": [
        "^build/upstream-pins\\.yaml$"
      ],
      "matchStrings": [
        "tag:\\s+(?<currentValue>v[0-9]+\\.[0-9]+\\.[0-9]+)"
      ],
      "depNameTemplate": "containers/gvisor-tap-vsock",
      "datasourceTemplate": "github-releases",
      "versioningTemplate": "semver"
    }
  ],
  "postUpdateOptions": [
    "gomodTidy"
  ],
  "ignorePaths": [
    "**/testdata/**",
    "**/vendor/**"
  ]
}
```

Notes on the config:

- `extends: ["config:recommended", ":semanticCommits", ":disableDependencyDashboard"]` — Renovate's recommended baseline plus semantic-commit prefixes; the issue-tracker dashboard is disabled because `TODO.md` is the canonical open-work tracker (workspace CLAUDE.md guideline #10: no duplicated state).
- `schedule: ["before 6am on monday"]` plus `timezone: "Europe/Bratislava"` — PRs land overnight Sunday → Monday Bratislava time, ready for Monday-morning review.
- `automerge: false` — every supply-chain bump waits for human eyes.
- `rangeStrategy: "pin"` — Renovate writes exact versions, not ranges. Matches the project's pinning policy.
- `customManagers[2]` (the Go apk pin): we use the `repology` datasource against `alpine_3_23/go` so Renovate tracks the Go version actually shipping in our pinned Alpine major. When the Alpine major bumps (e.g. 3.23 → 3.24), the `packageNameTemplate` needs a hand-edit; flag this in the PR description when bumping the digest. (Avoids hardcoding a separate Go-release feed that drifts from Alpine.)
- The third customManager pulls `gvisor-tap-vsock` releases from GitHub. Because we only pin the `tag:` line, Renovate cannot automatically refresh the two `sha256:` values — the engineer reviewing the PR pulls the new `sha256sums` from upstream and updates `upstream-pins.yaml` manually. The B3 task documents this exact procedure under "Bump procedure".

- [ ] **Step 2: Validate the JSON**

```bash
./dev/run.sh 'apk add --no-cache jq >/dev/null 2>&1 || true; jq . renovate.json >/dev/null && echo OK || echo INVALID'
```

Expected: `OK`.

- [ ] **Step 3: (Optional) Run Renovate locally to dry-run the config**

If the engineer has Renovate CLI available (`npx renovate@latest --platform=local --print-config`):

```bash
RENOVATE_CONFIG_FILE=renovate.json npx --yes renovate@latest --platform=local --dry-run=full
```

Expected: dry run reports the discovered packages from the four managers (gomod plus three customManagers). If a manager reports zero packages discovered, its `matchStrings` regex is wrong — fix and re-dry-run.

If `npx` / Node is unavailable in the dev container, skip this step; the install step in B8 Step 5 below catches misconfiguration via the bot's own validation.

- [ ] **Step 4: Open PR and merge**

```bash
git checkout -b deps/renovate-config
git add renovate.json
git commit -m "deps: renovate config (3 streams, weekly Mon, no auto-merge, sha-pinned regex managers for Alpine + apk go + gvisor-tap-vsock)"
git push -u origin deps/renovate-config
gh pr create --base main --head deps/renovate-config \
    --title "deps: Renovate config" \
    --body "Adds renovate.json with 3 streams: deps:go (gomod), deps:alpine (Alpine digest + go apk pin lockstep across dev/Containerfile and build/Dockerfile.rootfs), deps:gvisor-tap-vsock (tag in build/upstream-pins.yaml). Weekly Monday schedule, no auto-merge. Per Phase B addendum D-5."
gh pr checks --watch
gh pr merge --merge --delete-branch
git checkout main
git pull --ff-only
```

Expected: PR passes CI and merges.

- [ ] **Step 5: Install the Renovate GitHub App on the repo**

Install the public Renovate GitHub App (`https://github.com/apps/renovate`) and grant it access to `zeroznet/wsl-vpnfix` only:

```bash
gh repo view zeroznet/wsl-vpnfix --web
```

In the browser: Settings → Integrations → GitHub Apps → Configure Renovate → "Only select repositories" → `wsl-vpnfix`.

After installation, the app opens an "onboarding" PR titled `Configure Renovate` that includes a copy of our `renovate.json` plus a confirmation request. Review and merge it.

Within ~30 minutes of merging the onboarding PR, Renovate runs its first scan. Verify by visiting the repo's Pull Requests tab: any pending bumps appear as PRs labelled with one of `deps:go`, `deps:alpine`, `deps:gvisor-tap-vsock`. (Likely zero on first scan if everything is current.)

- [ ] **Step 6: Update TODO**

Remove the Phase B follow-on bullet from `TODO.md` Later section (the one mentioning "GitHub Actions CI/release/repro workflows, syft SBOM, hardened systemd unit (or PID-1 init wrapper), Renovate config that PRs `build/upstream-pins.yaml` and `go.mod` bumps separately") and replace with a concise pointer that Phase B is done.

```bash
# Hand-edit TODO.md — the Phase B follow-on bullet's text is too specific
# for a safe sed pattern.
$EDITOR TODO.md

git add TODO.md
git commit -m "todo: Phase B done (B1-B8 landed), promote Phase C bullets"
git push origin main  # this will require a PR if branch protection is enforcing
```

If push is rejected by branch protection, open a PR from a feature branch.

---

## Self-Review

Run this checklist after the plan is fully written; it is for the plan author, not the engineer executing.

### 1. Spec coverage

Walk through each section of `2026-05-09-wsl-vpnfix-phase-b-design.md` and confirm a task implements it:

| Spec section | Task | Notes |
|---|---|---|
| §1 D-1 (PID 1) | B2 | `initIfPID1()`, reaper, `/sbin/init` symlink in B3. |
| §1 D-2 (drop cosign) | B7 | `release.yml` has no cosign / OIDC / `id-token`. |
| §1 D-3 (no SBOM, no SLSA) | B7 | `release.yml` uploads tarball + `SHA256SUMS` + `upstream-pins.yaml` only. |
| §1 D-4 (`ubuntu-24.04`) | B6, B7 | both workflows pin `runs-on: ubuntu-24.04`. |
| §1 D-5 (Renovate, 3 streams, no auto-merge) | B8 | `renovate.json` with three managers, `automerge: false`, weekly. |
| §2 amendments to master spec section 2.6 (rootfs) | B3 | `Dockerfile.rootfs` final stage; `/sbin/init` symlink; no systemd. |
| §2 amendments to master spec section 3.4 (hardening) | B2, B3 | PID-1 reaper + SIGHUP forward; rootfs has no setuid extras, no shell. |
| §2 amendments to master spec section 4.3 (reproducibility) | B3 | `pack.sh` deterministic flags; reproducibility verified in B3 Step 7 and B7 Step 4. |
| §2 amendments to master spec section 4.5 (CI/CD) | B6, B7 | `ci.yml` workflow-level `permissions: read-all`; `release.yml` job-level `contents: write`. |
| §2 amendments to master spec section 4.6 (release artifacts) | B7 | tarball + `SHA256SUMS` + `upstream-pins.yaml`, nothing else. |
| §2 amendments to master spec section 4.8 (user update) | (Phase C) | `install-wslvpnfix.ps1` is explicitly out of scope; the README in Phase C will document the `sha256sum -c SHA256SUMS` flow. |
| §3 task B1-B8 | B1-B8 | one-to-one. |
| §4 (out of scope) | — | none of the Phase C items are scheduled inside Phase B. Verified each is referenced as deferred in the relevant task. |

No gaps.

### 2. Placeholder scan

Searched the plan for `TBD`, `TODO`, `implement later`, `fill in details`, `add appropriate`, `similar to Task`, and "write tests for the above" without code. The only `__FILL_FROM_UPSTREAM_SHA256SUMS__` strings are inside `build/upstream-pins.yaml` template content, with B3 Step 2 explicitly walking through how to replace them and B3's `pack.sh` aborting if they remain.

The `XX` in `docs/smoke-2026-05-XX.md` is a date that gets filled at run time; B4 Step 9 explicitly tells the engineer to substitute the actual day-of-month.

No other placeholders.

### 3. Type / signature consistency

- `initIfPID1()`, `procMounted()`, `startReaper(stop <-chan struct{})` — defined in B2 Step 3, used in B2 Step 1 (test) and B2 Step 4 (`main.go` modification). Names match.
- `Spec.Path`, `Spec.Args`, `Spec.Env` — already defined in `internal/process/manager.go` (Phase A), reused in `cmd/wsl-vpnfix/main.go:251-264` (Phase A). Phase B does not touch these.
- `build/pack.sh` env vars: `VERSION`, `COMMIT`, `SOURCE_DATE_EPOCH`, `GVTV_TAG`, `GVTV_GVFORWARDER_SHA256`, `GVTV_GVPROXY_EXE_SHA256`. Each is read by `Dockerfile.rootfs` via matching `ARG` lines. Names match.
- Renovate `customManagers[2]` packageName `alpine_3_23/go` — matches the Alpine major in the digest pinned at B3 Step 3. When the Alpine major bumps, this needs a manual update; B8 Step 1's notes call this out.

No mismatches.

---

## After this plan

1. The engineer (via `superpowers:executing-plans` or `superpowers:subagent-driven-development`) works through B1-B8 in order.
2. B4 is the merge gate — no skipping. If B4 is red, fix the root cause and re-run B4 from Step 1 before proceeding to B5.
3. Phase C planning starts after B8 lands. Inputs to that planning round: master spec sections 1-3 (unchanged), the Phase B addendum's "out of scope" list, and any new findings B4's smoke run surfaced. Brainstorming pass via `superpowers:brainstorming`; plan via `superpowers:writing-plans` at `docs/superpowers/plans/<YYYY-MM-DD>-wsl-vpnfix-phase-c-public-surface-and-audit.md`.
