// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

// Package netfilter constructs the nftables rules wsl-vpnfix needs
// and installs / removes them via the netlink-typed nftables library.
package netfilter

import (
	"errors"
	"fmt"
	"net"

	nft "github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

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

		// POSTROUTING: masquerade ALL traffic leaving the tap. Without
		// this, sibling distros (eth0 IP outside 192.168.127.0/24) send
		// to gvproxy with their original source IP; gvproxy's user-mode
		// stack cannot route a reply outside 192.168.127.0/24 and the
		// query times out. The earlier saddr-scoped variant (per spec
		// finding F-007's hypothetical "origin masking" concern)
		// silently broke DNS-via-resolv.conf for every sibling distro.
		// Verified on Win 11 25H2 + Ubuntu sibling on 2026-05-10.
		{DescTag: "masquerade", Chain: "postrouting", OutIface: p.TapName, Action: "masquerade"},
	}

	return RuleSet{Chains: chains, Rules: rules}, nil
}

// Install applies the rule set into a fresh nftables table named tableName,
// in the IPv4 family, replacing any existing table of that name. Idempotent:
// running it twice yields the same final state.
//
// The delete and create run on separate Conns (separate netlink batches) on
// purpose. Mixing DelTable + AddTable in a single transaction can abort the
// whole batch under nft atomic semantics when the delete targets a
// non-existent table; splitting them lets us tolerate ENOENT on the delete
// without forfeiting real errors on the create.
func Install(rs RuleSet, tableName string) error {
	// Step 1: best-effort drop of any existing table with this name.
	delConn := &nft.Conn{}
	delConn.DelTable(&nft.Table{Family: nft.TableFamilyIPv4, Name: tableName})
	if err := delConn.Flush(); err != nil && !errors.Is(err, unix.ENOENT) {
		return fmt.Errorf("nftables: stale table cleanup for %q: %w", tableName, err)
	}

	// Step 2: build the new table in its own batch.
	c := &nft.Conn{}
	tbl := c.AddTable(&nft.Table{Family: nft.TableFamilyIPv4, Name: tableName})

	chains := map[string]*nft.Chain{}
	for _, ch := range rs.Chains {
		hook, prio, ok := chainAttrs(ch.Name)
		if !ok {
			return fmt.Errorf("unknown chain %q", ch.Name)
		}
		chains[ch.Name] = c.AddChain(&nft.Chain{
			Name:     ch.Name,
			Table:    tbl,
			Type:     nft.ChainTypeNAT,
			Hooknum:  hook,
			Priority: prio,
		})
	}

	for _, r := range rs.Rules {
		ch, ok := chains[r.Chain]
		if !ok {
			return fmt.Errorf("rule %q: chain %q not declared", r.DescTag, r.Chain)
		}
		exprs, err := buildExprs(r)
		if err != nil {
			return fmt.Errorf("rule %q: %w", r.DescTag, err)
		}
		c.AddRule(&nft.Rule{Table: tbl, Chain: ch, Exprs: exprs})
	}

	return c.Flush()
}

// Remove deletes the named nftables table. Idempotent: a missing table is
// treated as success (ENOENT). Other errors propagate.
func Remove(tableName string) error {
	c := &nft.Conn{}
	c.DelTable(&nft.Table{Family: nft.TableFamilyIPv4, Name: tableName})
	if err := c.Flush(); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return fmt.Errorf("nftables remove %q: %w", tableName, err)
	}
	return nil
}

func chainAttrs(name string) (*nft.ChainHook, *nft.ChainPriority, bool) {
	switch name {
	case "prerouting":
		return nft.ChainHookPrerouting, nft.ChainPriorityNATDest, true
	case "output":
		return nft.ChainHookOutput, nft.ChainPriorityNATDest, true
	case "postrouting":
		return nft.ChainHookPostrouting, nft.ChainPriorityNATSource, true
	}
	return nil, nil, false
}

func buildExprs(r Rule) ([]expr.Any, error) {
	var out []expr.Any

	if r.MatchSrcCIDR != "" {
		_, cidr, err := net.ParseCIDR(r.MatchSrcCIDR)
		if err != nil {
			return nil, fmt.Errorf("invalid MatchSrcCIDR: %q: %w", r.MatchSrcCIDR, err)
		}
		network := cidr.IP.To4()
		if network == nil {
			return nil, fmt.Errorf("MatchSrcCIDR must be IPv4: %q", r.MatchSrcCIDR)
		}
		mask := []byte(cidr.Mask)
		if len(mask) != 4 {
			return nil, fmt.Errorf("MatchSrcCIDR mask must be 4 bytes (IPv4): %q", r.MatchSrcCIDR)
		}
		out = append(out,
			// Source IP = bytes 12..16 of the IPv4 network header.
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4},
			&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: mask, Xor: []byte{0, 0, 0, 0}},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: network},
		)
	}

	if r.MatchDst != "" {
		ip := net.ParseIP(r.MatchDst).To4()
		if ip == nil {
			return nil, fmt.Errorf("invalid MatchDst: %q", r.MatchDst)
		}
		out = append(out,
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 16, Len: 4},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ip},
		)
	}

	if r.MatchProto != "" {
		var proto byte
		switch r.MatchProto {
		case "tcp":
			proto = unix.IPPROTO_TCP
		case "udp":
			proto = unix.IPPROTO_UDP
		default:
			return nil, fmt.Errorf("unknown proto: %q", r.MatchProto)
		}
		out = append(out,
			&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{proto}},
		)
		if r.MatchPort != 0 {
			out = append(out,
				&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: portBytes(r.MatchPort)},
			)
		}
	}

	if r.OutIface != "" {
		out = append(out,
			&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ifname(r.OutIface)},
		)
	}

	switch r.Action {
	case "dnat":
		ip := net.ParseIP(r.DNATTo).To4()
		if ip == nil {
			return nil, fmt.Errorf("invalid DNATTo: %q", r.DNATTo)
		}
		out = append(out, &expr.Immediate{Register: 1, Data: ip})
		if r.DNATPort != 0 {
			out = append(out, &expr.Immediate{Register: 2, Data: portBytes(r.DNATPort)})
			out = append(out, &expr.NAT{Type: expr.NATTypeDestNAT, Family: unix.NFPROTO_IPV4, RegAddrMin: 1, RegProtoMin: 2})
		} else {
			out = append(out, &expr.NAT{Type: expr.NATTypeDestNAT, Family: unix.NFPROTO_IPV4, RegAddrMin: 1})
		}
	case "masquerade":
		out = append(out, &expr.Masq{})
	default:
		return nil, fmt.Errorf("unknown Action: %q", r.Action)
	}

	return out, nil
}

func portBytes(port int) []byte {
	return []byte{byte(port >> 8), byte(port)}
}

func ifname(name string) []byte {
	b := make([]byte, 16)
	copy(b, []byte(name))
	return b
}
