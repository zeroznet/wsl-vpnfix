// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Haiku 4.5)

// Package config defines the runtime configuration for wsl-vpnfix
// and the loader that builds it from environment variables.
package config

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
