// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

// Package process spawns child processes with a strict policy:
// absolute paths only, explicit env (nil = inherit, []string{} = empty),
// context-driven termination (SIGTERM then SIGKILL via WaitDelay).
package process

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// Spec describes a process to spawn.
type Spec struct {
	Path   string
	Args   []string
	Env    []string // nil = inherit parent env (Go's default); []string{} = empty
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Handle wraps a running child.
type Handle struct {
	cmd  *exec.Cmd
	err  error
	done chan struct{}
}

type Manager struct{}

func NewManager() *Manager { return &Manager{} }

// Spawn starts the process and returns a Handle. The child receives SIGTERM
// when ctx is cancelled; if it does not exit within WaitDelay, SIGKILL.
// Path must be absolute.
func (m *Manager) Spawn(ctx context.Context, s Spec) (*Handle, error) {
	if !filepath.IsAbs(s.Path) {
		return nil, fmt.Errorf("process: Path must be absolute, got %q", s.Path)
	}

	cmd := exec.CommandContext(ctx, s.Path, s.Args...)
	cmd.Env = s.Env
	cmd.Stdin = s.Stdin
	cmd.Stdout = s.Stdout
	cmd.Stderr = s.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Signal the entire process group, not just the leader. gvforwarder
	// spawns wsl-gvproxy.exe (via WSL interop) without its own Setpgid, so
	// the .exe inherits this group but is not the leader. SIGTERM to the
	// pid alone would leave the .exe orphaned. `kill -pgid` mirrors upstream
	// wsl-vpnkit's `kill 0`.
	done := make(chan struct{})
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		// WaitDelay's own escalation SIGKILLs only the group leader; a
		// SIGTERM-surviving non-leader (the interop stub the comment above
		// exists for) would outlive it. Escalate group-wide on the same
		// clock, skipped if the child already exited.
		time.AfterFunc(termGracePeriod, func() {
			select {
			case <-done:
			default:
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		})
		return err
	}
	cmd.WaitDelay = termGracePeriod

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("process: start %s: %w", s.Path, err)
	}

	h := &Handle{cmd: cmd, done: done}
	go func() { h.err = cmd.Wait(); close(done) }()
	return h, nil
}

const termGracePeriod = 5 * time.Second

// Wait blocks until the child exits and returns its exit error. Safe to
// call any number of times.
func (h *Handle) Wait() error { <-h.done; return h.err }

// Done returns a channel that is closed once the child has exited; the
// exit error is then available via Wait.
func (h *Handle) Done() <-chan struct{} { return h.done }

func (h *Handle) Pid() int {
	if h.cmd == nil || h.cmd.Process == nil {
		return -1
	}
	return h.cmd.Process.Pid
}
