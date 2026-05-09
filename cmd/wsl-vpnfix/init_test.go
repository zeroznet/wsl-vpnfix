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
