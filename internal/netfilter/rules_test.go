// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

package netfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validParams() Params {
	return Params{
		WSL2GatewayIP:   "172.20.16.1",
		VpnkitGatewayIP: "192.168.127.1",
		VpnkitHostIP:    "192.168.127.254",
		VpnkitLocalCIDR: "192.168.127.0/24",
		TapName:         "wsltap",
	}
}

func TestBuildRuleSet_HasExpectedChains(t *testing.T) {
	rs, err := BuildRuleSet(validParams())
	require.NoError(t, err)

	chainNames := map[string]bool{}
	for _, c := range rs.Chains {
		chainNames[c.Name] = true
	}
	assert.True(t, chainNames["prerouting"], "prerouting chain must exist")
	assert.True(t, chainNames["output"], "output chain must exist")
	assert.True(t, chainNames["postrouting"], "postrouting chain must exist")

	assert.NotEmpty(t, rs.Rules, "ruleset must contain rules")
}

func TestBuildRuleSet_RejectsInvalidInputs(t *testing.T) {
	cases := []struct {
		name string
		p    Params
	}{
		{"missing WSL2 gateway", func() Params { p := validParams(); p.WSL2GatewayIP = ""; return p }()},
		{"missing vpnkit gateway", func() Params { p := validParams(); p.VpnkitGatewayIP = ""; return p }()},
		{"missing host IP", func() Params { p := validParams(); p.VpnkitHostIP = ""; return p }()},
		{"missing tap name", func() Params { p := validParams(); p.TapName = ""; return p }()},
		{"missing CIDR", func() Params { p := validParams(); p.VpnkitLocalCIDR = ""; return p }()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := BuildRuleSet(c.p)
			assert.Error(t, err)
		})
	}
}

func TestBuildRuleSet_DNSRedirectsToVpnkitGateway(t *testing.T) {
	rs, err := BuildRuleSet(validParams())
	require.NoError(t, err)

	gotDNS := false
	for _, r := range rs.Rules {
		if r.DescTag == "dns-tcp" || r.DescTag == "dns-udp" {
			assert.Equal(t, "192.168.127.1", r.DNATTo)
			assert.Equal(t, 53, r.DNATPort)
			gotDNS = true
		}
	}
	assert.True(t, gotDNS, "expected DNS DNAT rules in ruleset")
}

func TestBuildRuleSet_MasqueradeScopedToTapAndSourceCIDR(t *testing.T) {
	rs, err := BuildRuleSet(validParams())
	require.NoError(t, err)

	gotMasq := false
	for _, r := range rs.Rules {
		if r.Action == "masquerade" {
			assert.Equal(t, "wsltap", r.OutIface)
			assert.Equal(t, "192.168.127.0/24", r.MatchSrcCIDR, "masquerade must be scoped to source CIDR (spec F-007)")
			gotMasq = true
		}
	}
	assert.True(t, gotMasq, "expected masquerade rule scoped to wsltap + source CIDR")
}
