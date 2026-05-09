// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

// Package netfilter constructs the nftables rules wsl-vpnfix needs
// and installs / removes them via the netlink-typed nftables library.
package netfilter

import "fmt"

type Params struct {
	WSL2GatewayIP   string
	VpnkitGatewayIP string
	VpnkitHostIP    string
	VpnkitLocalCIDR string
	TapName         string
}

type Chain struct {
	Name string
}

// Rule is a high-level rule descriptor. The actual netlink expressions
// are produced when this rule is installed; this layer is the unit
// of test and audit.
type Rule struct {
	DescTag      string // human-readable tag for tests/logs (e.g. "dns-tcp")
	Chain        string // "prerouting" | "output" | "postrouting"
	MatchSrcCIDR string // optional, e.g. "192.168.127.0/24" — saddr scope
	MatchDst     string // optional, IP or empty
	MatchProto   string // "tcp" | "udp" | ""
	MatchPort    int    // 0 if not used
	OutIface     string // for postrouting masquerade
	Action       string // "dnat" | "masquerade"
	DNATTo       string // dnat target IP
	DNATPort     int    // dnat target port (0 = preserve)
}

type RuleSet struct {
	Chains []Chain
	Rules  []Rule
}

// BuildRuleSet assembles the rule set from Params. Returns an error if
// any required field is empty.
func BuildRuleSet(p Params) (RuleSet, error) {
	switch {
	case p.WSL2GatewayIP == "":
		return RuleSet{}, fmt.Errorf("BuildRuleSet: WSL2GatewayIP is required")
	case p.VpnkitGatewayIP == "":
		return RuleSet{}, fmt.Errorf("BuildRuleSet: VpnkitGatewayIP is required")
	case p.VpnkitHostIP == "":
		return RuleSet{}, fmt.Errorf("BuildRuleSet: VpnkitHostIP is required")
	case p.VpnkitLocalCIDR == "":
		return RuleSet{}, fmt.Errorf("BuildRuleSet: VpnkitLocalCIDR is required")
	case p.TapName == "":
		return RuleSet{}, fmt.Errorf("BuildRuleSet: TapName is required")
	}

	chains := []Chain{
		{Name: "prerouting"},
		{Name: "output"},
		{Name: "postrouting"},
	}

	rules := []Rule{
		// PREROUTING: redirect WSL2 gateway DNS to vpnkit gateway DNS
		{DescTag: "dns-udp", Chain: "prerouting", MatchDst: p.WSL2GatewayIP, MatchProto: "udp", MatchPort: 53, Action: "dnat", DNATTo: p.VpnkitGatewayIP, DNATPort: 53},
		{DescTag: "dns-tcp", Chain: "prerouting", MatchDst: p.WSL2GatewayIP, MatchProto: "tcp", MatchPort: 53, Action: "dnat", DNATTo: p.VpnkitGatewayIP, DNATPort: 53},
		// PREROUTING: any other packet to WSL2 gateway -> vpnkit host
		{DescTag: "host-prerouting", Chain: "prerouting", MatchDst: p.WSL2GatewayIP, Action: "dnat", DNATTo: p.VpnkitHostIP},

		// OUTPUT: same redirects for locally-generated traffic
		{DescTag: "dns-udp-out", Chain: "output", MatchDst: p.WSL2GatewayIP, MatchProto: "udp", MatchPort: 53, Action: "dnat", DNATTo: p.VpnkitGatewayIP, DNATPort: 53},
		{DescTag: "dns-tcp-out", Chain: "output", MatchDst: p.WSL2GatewayIP, MatchProto: "tcp", MatchPort: 53, Action: "dnat", DNATTo: p.VpnkitGatewayIP, DNATPort: 53},
		{DescTag: "host-output", Chain: "output", MatchDst: p.WSL2GatewayIP, Action: "dnat", DNATTo: p.VpnkitHostIP},

		// POSTROUTING: masquerade traffic leaving the tap, scoped to our
		// vpnkit private network as source. Belt-and-braces with the
		// oifname match — the saddr scope addresses spec finding F-007.
		{DescTag: "masquerade", Chain: "postrouting", OutIface: p.TapName, MatchSrcCIDR: p.VpnkitLocalCIDR, Action: "masquerade"},
	}

	return RuleSet{Chains: chains, Rules: rules}, nil
}
