// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Haiku 4.5)

// Package config defines the runtime configuration for wsl-vpnfix
// and the loader that builds it from environment variables.
package config

import (
	"fmt"
	"os"
)

// VpnkitGatewayIP, VpnkitHostIP, VpnkitLocalIP, VpnkitLocalCIDR, and
// TapPrefixLen are package-level constants — not env-overridable — because
// gvproxy v0.8.8 has the host IP (192.168.127.254) hardcoded in its DNS
// records for `host.containers.internal` and the .2:5a:94:ef:e4:0c:ee
// mapping hardcoded in its DHCP static lease map
// (cmd/gvproxy/config.go:22-23, 372). Those defaults only apply in the
// CLI mode (no -config file) — which is exactly the mode gvforwarder
// spawns gvproxy in via its `stdio:` URL scheme. Changing the subnet at
// runtime would silently leave host.containers.internal pointing at
// 192.168.127.254 outside the new subnet, partially breaking the wiring.
//
// 192.168.127.0/24 is also a sound choice for 2026: RFC 1918, an unusual
// third octet (.127) that home routers and corporate VPNs almost never
// use, and not in 100.64.0.0/10 which collides with Tailscale.
const (
	VpnkitGatewayIP = "192.168.127.1"
	VpnkitHostIP    = "192.168.127.254"
	VpnkitLocalIP   = "192.168.127.2"
	VpnkitLocalCIDR = "192.168.127.0/24"
	TapPrefixLen    = 24
)

// Config holds the runtime knobs for wsl-vpnfix that are env-overridable.
// The IP plan above is intentionally NOT here — see the constants block.
type Config struct {
	WSL2GatewayIP   string // optional override; empty = autodetect from resolv.conf
	TapMACAddr      string
	TapName         string
	GvproxyPath     string
	GvforwarderPath string
	CheckHost       string
	CheckDNS        string
	Debug           bool
}

func Default() Config {
	return Config{
		WSL2GatewayIP:   "", // autodetect by default
		TapMACAddr:      "5a:94:ef:e4:0c:ee",
		TapName:         "wsltap",
		GvproxyPath:     "/etc/wsl-vpnfix/wsl-gvproxy.exe",
		GvforwarderPath: "/sbin/wsl-gvforwarder",
		CheckHost:       "example.com",
		CheckDNS:        "1.1.1.1",
		Debug:           false,
	}
}

// Load builds a Config by starting from Default() and applying
// any non-empty matching environment variables. Every override
// is validated; any invalid value returns an error and aborts.
func Load() (Config, error) {
	c := Default()

	type field struct {
		envName  string
		dst      *string
		validate func(string) error
	}
	stringFields := []field{
		{"WSL2_GATEWAY_IP", &c.WSL2GatewayIP, ValidateIPv4},
		{"TAP_MAC_ADDR", &c.TapMACAddr, ValidateMAC},
		{"TAP_NAME", &c.TapName, ValidateInterfaceName},
		{"GVPROXY_PATH", &c.GvproxyPath, ValidateAbsolutePath},
		{"GVFORWARDER_PATH", &c.GvforwarderPath, ValidateAbsolutePath},
		{"CHECK_HOST", &c.CheckHost, ValidateHostname},
		{"CHECK_DNS", &c.CheckDNS, ValidateIPv4},
	}
	for _, f := range stringFields {
		v := os.Getenv(f.envName)
		if v == "" {
			continue
		}
		if err := f.validate(v); err != nil {
			return Config{}, fmt.Errorf("env %s: %w", f.envName, err)
		}
		*f.dst = v
	}

	switch os.Getenv("DEBUG") {
	case "":
		// keep default
	case "0", "false", "FALSE", "False":
		c.Debug = false
	case "1", "true", "TRUE", "True":
		c.Debug = true
	default:
		return Config{}, fmt.Errorf("env DEBUG: invalid bool value %q", os.Getenv("DEBUG"))
	}

	return c, nil
}
