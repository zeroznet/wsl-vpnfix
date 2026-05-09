// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpawn_TrueExitsZero(t *testing.T) {
	m := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := m.Spawn(ctx, Spec{Path: "/bin/true"})
	require.NoError(t, err)

	err = h.Wait()
	assert.NoError(t, err)
}

func TestSpawn_FalseExitsNonZero(t *testing.T) {
	m := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := m.Spawn(ctx, Spec{Path: "/bin/false"})
	require.NoError(t, err)

	err = h.Wait()
	var exitErr *exec.ExitError
	assert.True(t, errors.As(err, &exitErr))
	assert.Equal(t, 1, exitErr.ExitCode())
}

func TestSpawn_RejectsRelativePath(t *testing.T) {
	m := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := m.Spawn(ctx, Spec{Path: "true"})
	assert.Error(t, err)
}

func TestSpawn_TerminatesOnContextCancel(t *testing.T) {
	if os.Getenv("CI_SKIP_SLEEP") == "1" {
		t.Skip("skipped under CI_SKIP_SLEEP")
	}
	m := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	h, err := m.Spawn(ctx, Spec{Path: "/bin/sleep", Args: []string{"30"}})
	require.NoError(t, err)

	err = h.Wait()
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || isSignaledKill(err), "got: %v", err)
}

// TestSpawn_TerminatesProcessGroup confirms that cancelling the context
// signals the entire process group, not just the leader pid. gvforwarder
// spawns wsl-gvproxy.exe without its own Setpgid, so a leader-only signal
// would orphan the .exe.
func TestSpawn_TerminatesProcessGroup(t *testing.T) {
	if os.Getenv("CI_SKIP_SLEEP") == "1" {
		t.Skip("skipped under CI_SKIP_SLEEP")
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	m := NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parent shell backgrounds /bin/sleep, records its pid, waits.
	// If we signal only the leader (the shell), the grandchild lives.
	// If we signal the pgroup, both die.
	h, err := m.Spawn(ctx, Spec{
		Path: "/bin/sh",
		Args: []string{"-c", "/bin/sleep 30 & echo $! > " + pidFile + "; wait"},
	})
	require.NoError(t, err)

	var grandPid int
	require.Eventually(t, func() bool {
		b, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		s := strings.TrimSpace(string(b))
		if s == "" {
			return false
		}
		p, err := strconv.Atoi(s)
		if err != nil {
			return false
		}
		grandPid = p
		return true
	}, 2*time.Second, 20*time.Millisecond, "grandchild pid file never appeared")

	cancel()
	_ = h.Wait()

	// kernel needs a tick to reap.
	time.Sleep(200 * time.Millisecond)

	// On a fully reaped grandchild, kill(pid, 0) returns ESRCH. In a pid
	// namespace whose PID 1 does not reap orphans (rootless podman dev
	// container), the SIGTERM kills the process but it lingers as a
	// zombie until the container exits. Either outcome proves the pgroup
	// signal reached the grandchild; only "alive and Running/Sleeping"
	// is a real failure.
	err = syscall.Kill(grandPid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return
	}
	require.NoError(t, err, "kill(pid, 0) returned unexpected error for pid %d", grandPid)
	statusBytes, statErr := os.ReadFile(fmt.Sprintf("/proc/%d/status", grandPid))
	require.NoError(t, statErr, "could not read /proc/%d/status to verify grandchild state", grandPid)
	assert.Contains(t, string(statusBytes), "State:\tZ",
		"grandchild pid %d is still alive (not zombie); pgroup signal failed:\n%s", grandPid, statusBytes)
}

func isSignaledKill(err error) bool {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return false
	}
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
		return ws.Signaled() && (ws.Signal() == syscall.SIGTERM || ws.Signal() == syscall.SIGKILL)
	}
	return false
}
