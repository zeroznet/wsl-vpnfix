<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 5.5) -->

# wsl-vpnfix gvproxy high-resolution clock: Design

**Date:** 2026-11-18
**Status:** Approved (option B), not started
**Target release:** v0.3.0

## Problem

The stdio bridge carries 7x to 30x the payload: 84% of segments the guest receives are duplicates (`docs/smoke-2026-10-04.md`, `docs/smoke-2026-11-18.md`). Root cause, reproduced on Linux on 2026-11-18:

- Go's monotonic clock on windows/amd64 reads `KUSER_SHARED_DATA` InterruptTime (`runtime/time_windows_amd64.s`, `_INTERRUPT_TIME 0x7ffe0008`), which advances only per system tick (15.625 ms default).
- gvproxy builds its gVisor stack without `stack.Options.Clock`, so gVisor uses `tcpip.NewStdClock()` on top of that clock.
- On a ~1 ms bridge most RTT samples are 0. RACK (`pkg/tcpip/transport/tcp/rack.go`, gVisor `20260413`) then has `minRTT` = `RTT` = `ReoWnd` = 0 and marks in-flight segments lost en masse; DSACK cannot widen `ReoWnd = minRTT/4 * incr` while `minRTT` is 0.

Evidence (Linux gvproxy v0.9.0 over the real stdio bridge, 10 MB download):

| gvproxy build | speed | DSACKOldSent / InSegs | wire bytes |
|---|---|---|---|
| unmodified | 89 MB/s | 0 | 11 MB |
| stdclock truncated to 15.625 ms | 2.5 MB/s | 97% | 335 MB |
| truncated stdclock, RACK off | 95 MB/s | 0 | 11 MB |

## Decisions

| ID | Decision | Rationale |
|---|---|---|
| HC-D-1 | Build `gvproxy-windowsgui.exe` from the pinned upstream tag plus one local patch; `gvforwarder` stays the upstream release binary verified by SHA-256 | The bug lives only in gvproxy.exe. gvforwarder runs no gVisor TCP stack. |
| HC-D-2 | The patch gives gVisor a QueryPerformanceCounter clock via `stack.Options.Clock` on Windows; other OSes keep the default clock | Fixes the cause (clock resolution), not the symptom (RACK). Disabling RACK would hide the bug and degrade real-loss recovery on lossy VPN paths. |
| HC-D-3 | Source pinned by tag and commit SHA; the build fails if the clone's `HEAD` differs from the pinned commit or the patch does not apply | Replaces the artifact SHA-256 as the integrity anchor. A tag-only Renovate bump turns CI red, same pattern as RR-D-1. |
| HC-D-4 | The patched gvproxy reports version `<tag>-wsl-vpnfix` | Logs and support tell a patched build from the upstream artifact. |
| HC-D-5 | The patch is temporary. An upstream issue and PR go out (drafted for Robert to post), and the patch is removed when a gvisor-tap-vsock release carries an equivalent fix | Keeps us on upstream artifacts long term. |
| HC-D-6 | Release as v0.3.0 | The shipped gvproxy is no longer the upstream artifact: a supply-chain shape change, not a patch-level bump. |

## Out of scope

Other v0.3.0 bundle items get their own plans. arm64 gvproxy is out of scope (the appliance ships amd64 only).
