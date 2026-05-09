// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

//go:build integration

package netlink

import (
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vnl "github.com/vishvananda/netlink"
)

func skipIfNotRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("integration test requires root / CAP_NET_ADMIN")
	}
}

func TestTap_CreateConfigureUpDown_Cycle(t *testing.T) {
	skipIfNotRoot(t)

	const (
		name = "wsltest0"
		mac  = "5a:94:ef:e4:0c:ff"
		ip   = "192.168.127.2"
	)

	defer func() { _ = DeleteTap(name) }()

	require.NoError(t, CreateTap(name, mac))

	link, err := vnl.LinkByName(name)
	require.NoError(t, err)
	assert.Equal(t, name, link.Attrs().Name)
	assert.Equal(t, net.HardwareAddr{0x5a, 0x94, 0xef, 0xe4, 0x0c, 0xff}, link.Attrs().HardwareAddr)

	require.NoError(t, AddAddr(name, ip, 24))
	require.NoError(t, SetUp(name))

	link, err = vnl.LinkByName(name)
	require.NoError(t, err)
	assert.NotZero(t, link.Attrs().Flags&net.FlagUp, "tap should be up")

	require.NoError(t, DeleteTap(name))
	_, err = vnl.LinkByName(name)
	assert.Error(t, err)
}

func TestTap_Idempotent(t *testing.T) {
	skipIfNotRoot(t)

	const (
		name = "wsltest_idem"
		mac  = "5a:94:ef:e4:0c:fd"
	)
	defer func() { _ = DeleteTap(name) }()

	require.NoError(t, CreateTap(name, mac))
	require.NoError(t, CreateTap(name, mac), "second create with same MAC must succeed")

	const otherMac = "5a:94:ef:e4:0c:fc"
	err := CreateTap(name, otherMac)
	assert.Error(t, err, "create with different MAC must fail")
}

func TestRoute_AddDefaultViaTap(t *testing.T) {
	skipIfNotRoot(t)

	const (
		name = "wsltest1"
		mac  = "5a:94:ef:e4:0c:fe"
		ip   = "192.168.127.2"
		gw   = "192.168.127.1"
	)
	defer func() { _ = DeleteTap(name) }()

	require.NoError(t, CreateTap(name, mac))
	require.NoError(t, AddAddr(name, ip, 24))
	require.NoError(t, SetUp(name))
	require.NoError(t, AddDefaultRoute(name, gw))

	routes, err := vnl.RouteList(nil, vnl.FAMILY_V4)
	require.NoError(t, err)

	found := false
	for _, r := range routes {
		if isDefaultDst(r.Dst) && r.Gw != nil && r.Gw.String() == gw {
			found = true
			break
		}
	}
	assert.True(t, found, "default route via %s not found", gw)
}
