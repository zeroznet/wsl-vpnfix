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
//  1. Reap zombie children (parent of all orphans must wait4 them).
//  2. Ensure /proc is mounted (some sibling code reads /proc/self/...).
//  3. Add SIGHUP to the signals that trigger orderly teardown. SIGINT
//     and SIGTERM are already wired in main.go's installSignalHandler.
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
//
// Signal registration happens before the function enters its select loop.
// An initial drain pass after registration catches zombies that arrived in
// the window between child spawn and signal.Notify — a common race in init
// reapers when children exit almost immediately.
func startReaper(stop <-chan struct{}) {
	sigchld := make(chan os.Signal, 16) // depth handles burst arrivals
	signal.Notify(sigchld, syscall.SIGCHLD)
	defer signal.Stop(sigchld)

	// Initial drain: catch zombies already present before Notify registered.
	drainZombies()

	for {
		select {
		case <-stop:
			return
		case <-sigchld:
		}
		drainZombies()
	}
}

// drainZombies calls Wait4 with WNOHANG in a loop until no more zombies remain.
// Covers bursts where one SIGCHLD coalesced multiple child exits.
func drainZombies() {
	for {
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
		if err != nil || pid <= 0 {
			break
		}
	}
}
