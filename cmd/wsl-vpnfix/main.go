// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

// Command wsl-vpnfix is the orchestrator for the wsl-vpnfix appliance distro.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zeroznet/wsl-vpnfix/internal/config"
	"github.com/zeroznet/wsl-vpnfix/internal/healthcheck"
	"github.com/zeroznet/wsl-vpnfix/internal/netfilter"
	"github.com/zeroznet/wsl-vpnfix/internal/netlink"
	"github.com/zeroznet/wsl-vpnfix/internal/process"
	"github.com/zeroznet/wsl-vpnfix/internal/wsl"
)

const (
	tableName        = "wsl-vpnfix"
	teardownStepWait = 10 * time.Second
)

var (
	version = "dev"
	commit  = "none"

	// envAllowlist is the explicit set of env vars copied to gvforwarder.
	// WSL_INTEROP: the path to the interop socket; required for spawning
	//   wsl-gvproxy.exe via WSL's binfmt_misc handler.
	// PATH: required by /init's lookup helpers and by the binfmt handler.
	// WSL_DISTRO_NAME: identifies the appliance distro to /init.
	// WSLENV: cross-OS env-propagation rules; preserves any user-set
	//   forwarding configured for the .exe (e.g. proxy/cert env).
	envAllowlist = []string{"WSL_INTEROP", "PATH", "WSL_DISTRO_NAME", "WSLENV"}
)

func main() {
	printConfig := flag.Bool("print-config", false, "print resolved config as JSON and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("wsl-vpnfix %s (%s)\n", version, commit)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fatal("config: %s", err)
	}

	if *printConfig {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cfg); err != nil {
			fatal("encode: %s", err)
		}
		return
	}

	if os.Geteuid() != 0 {
		fatal("must run as root (need CAP_NET_ADMIN for tap and nftables)")
	}

	if err := run(cfg); err != nil {
		fatal("%s", err)
	}
}

// teardown is an LIFO stack of cleanup closures. Each setup step appends to
// it on success; on signal or error we walk it in reverse and run every step
// with a bounded timeout. Survives signals received during init.
type teardown struct {
	mu    sync.Mutex
	steps []teardownStep
}

type teardownStep struct {
	name string
	fn   func() error
}

func (t *teardown) push(name string, fn func() error) {
	t.mu.Lock()
	t.steps = append(t.steps, teardownStep{name: name, fn: fn})
	t.mu.Unlock()
}

func (t *teardown) runAll() {
	t.mu.Lock()
	steps := append([]teardownStep(nil), t.steps...)
	t.steps = nil
	t.mu.Unlock()

	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		done := make(chan struct{})
		go func() {
			if err := s.fn(); err != nil {
				logf("teardown %s: %s", s.name, err)
			}
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(teardownStepWait):
			logf("teardown %s: timed out after %s", s.name, teardownStepWait)
		}
	}
}

// run wires the orchestrator: signal handling, six setup phases, child
// spawn, healthchecks, steady-state. The order of phase calls below is
// the contract — each phase's teardown closure is pushed inside the
// phase, so reordering would silently break partial-init recovery.
func run(cfg config.Config) error {
	td := &teardown{}
	defer td.runAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	installSignalHandler(cancel)

	wsl2GW, err := resolveWSL2GatewayIP(cfg)
	if err != nil {
		return err
	}

	if err := captureDefaults(td); err != nil {
		return err
	}
	if err := bringUpTap(td, cfg); err != nil {
		return err
	}
	if err := installTapDefaultRoute(td, cfg); err != nil {
		return err
	}
	if err := installNATRules(td, cfg, wsl2GW); err != nil {
		return err
	}

	fw, err := spawnGvforwarder(ctx, cfg)
	if err != nil {
		return err
	}

	startHealthchecks(ctx, cfg)
	waitForExit(ctx, cancel, fw)
	return nil
}

func installSignalHandler(cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sigs
		logf("signal %s, tearing down", s)
		cancel()
	}()
}

func resolveWSL2GatewayIP(cfg config.Config) (string, error) {
	if cfg.WSL2GatewayIP != "" {
		logf("WSL2 gateway IP (override via WSL2_GATEWAY_IP): %s", cfg.WSL2GatewayIP)
		return cfg.WSL2GatewayIP, nil
	}
	detected, err := wsl.WSL2GatewayIP()
	if err != nil {
		return "", fmt.Errorf("detect WSL2 gateway: %w", err)
	}
	logf("WSL2 gateway IP (autodetected): %s", detected)
	return detected, nil
}

// captureDefaults captures and removes existing IPv4 default routes so
// our tap default can be installed cleanly. The original routes go into
// a teardown closure that restores them verbatim — no hardcoded eth0
// fallback, so a user with policy routing or a non-default NIC name is
// restored to their actual prior state.
func captureDefaults(td *teardown) error {
	snap, err := netlink.CaptureAndDelDefaultRoutes()
	if err != nil {
		return fmt.Errorf("capture/clear existing default route: %w", err)
	}
	if snap.Len() > 0 {
		td.push("restore-original-default-routes", func() error {
			return netlink.RestoreRoutes(snap)
		})
	}
	return nil
}

func bringUpTap(td *teardown, cfg config.Config) error {
	if err := netlink.CreateTap(cfg.TapName, cfg.TapMACAddr); err != nil {
		return fmt.Errorf("tap create: %w", err)
	}
	td.push("delete-tap", func() error { return netlink.DeleteTap(cfg.TapName) })

	if err := netlink.AddAddr(cfg.TapName, config.VpnkitLocalIP, config.TapPrefixLen); err != nil {
		return fmt.Errorf("tap addr: %w", err)
	}
	if err := netlink.SetUp(cfg.TapName); err != nil {
		return fmt.Errorf("tap up: %w", err)
	}
	return nil
}

func installTapDefaultRoute(td *teardown, cfg config.Config) error {
	if err := netlink.AddDefaultRoute(cfg.TapName, config.VpnkitGatewayIP); err != nil {
		return fmt.Errorf("default route: %w", err)
	}
	td.push("delete-tap-default-route", func() error {
		return netlink.DelDefaultRoute(cfg.TapName, config.VpnkitGatewayIP)
	})
	return nil
}

func installNATRules(td *teardown, cfg config.Config, wsl2GW string) error {
	rs, err := netfilter.BuildRuleSet(netfilter.Params{
		WSL2GatewayIP:   wsl2GW,
		VpnkitGatewayIP: config.VpnkitGatewayIP,
		VpnkitHostIP:    config.VpnkitHostIP,
		VpnkitLocalCIDR: config.VpnkitLocalCIDR,
		TapName:         cfg.TapName,
	})
	if err != nil {
		return fmt.Errorf("nftables build: %w", err)
	}
	if err := netfilter.Install(rs, tableName); err != nil {
		return fmt.Errorf("nftables install: %w", err)
	}
	td.push("nftables-remove", func() error { return netfilter.Remove(tableName) })
	return nil
}

// spawnGvforwarder launches gvforwarder as our single child. The forwarder
// spawns wsl-gvproxy.exe itself via its `stdio:` URL scheme — we never
// spawn the .exe directly.
func spawnGvforwarder(ctx context.Context, cfg config.Config) (*process.Handle, error) {
	debugFlag := boolStr(cfg.Debug)
	stdioURL := fmt.Sprintf("stdio:%s?listen-stdio=accept&debug=%s", cfg.GvproxyPath, debugFlag)
	spec := process.Spec{
		Path: cfg.GvforwarderPath,
		Args: []string{
			"-url=" + stdioURL,
			"-iface=" + cfg.TapName,
			"-stop-if-exist=",
			"-preexisting=1",
			"-mac=" + cfg.TapMACAddr,
			"-debug=" + debugFlag,
		},
		Env:    buildEnv(envAllowlist),
		Stdout: os.Stderr,
		Stderr: os.Stderr,
	}
	logf("spawning gvforwarder: %s", strings.Join(spec.Args, " "))
	h, err := process.NewManager().Spawn(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("spawn gvforwarder: %w", err)
	}
	return h, nil
}

func startHealthchecks(ctx context.Context, cfg config.Config) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
		logf("health: %v", healthcheck.ProbeDNS(ctx, cfg.CheckHost, cfg.CheckDNS, 3*time.Second))
		logf("health: %v", healthcheck.ProbeHTTP(ctx, "https://"+cfg.CheckHost, 5*time.Second, nil))
	}()
}

// waitForExit blocks until the forwarder exits or a signal cancels ctx.
// gvforwarder runs a forever-loop, so a clean voluntary exit is unusual;
// treat any exit as fault and tear down.
func waitForExit(ctx context.Context, cancel context.CancelFunc, fw *process.Handle) {
	select {
	case err := <-fw.Done():
		logf("gvforwarder exited: %v (forever-loop should not exit voluntarily)", err)
	case <-ctx.Done():
		logf("ctx cancelled, waiting for forwarder to exit")
	}

	cancel()
	select {
	case <-fw.Done():
	case <-time.After(teardownStepWait):
		logf("gvforwarder did not exit within %s after cancel", teardownStepWait)
	}
}

// buildEnv constructs a child env block containing only those variables
// in `allowlist` that are set in the parent env. Returns nil for an
// empty allowlist; see process.Spec docs for nil vs empty Env semantics.
func buildEnv(allowlist []string) []string {
	if len(allowlist) == 0 {
		return nil
	}
	out := make([]string, 0, len(allowlist))
	for _, k := range allowlist {
		v, ok := os.LookupEnv(k)
		if !ok {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "wsl-vpnfix: "+format+"\n", args...)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "wsl-vpnfix: "+format+"\n", args...)
	os.Exit(1)
}
