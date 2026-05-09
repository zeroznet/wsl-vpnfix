// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Haiku 4.5)

// Command wsl-vpnfix is the orchestrator for the wsl-vpnfix appliance distro.
// It validates config, sets up tap + nftables, spawns gvforwarder, runs
// healthchecks, and tears everything down on signal.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/zeroznet/wsl-vpnfix/internal/config"
)

var (
	version = "dev"
	commit  = "none"
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
		fmt.Fprintf(os.Stderr, "config: %s\n", err)
		os.Exit(2)
	}

	if *printConfig {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "encode: %s\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintln(os.Stderr, "wsl-vpnfix: orchestrator wiring not yet implemented (Task 13)")
	os.Exit(1)
}
