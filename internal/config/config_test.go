// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Haiku 4.5)

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig_Values(t *testing.T) {
	c := Default()

	assert.Equal(t, "", c.WSL2GatewayIP, "default WSL2GatewayIP must be empty (autodetect)")
	assert.Equal(t, "5a:94:ef:e4:0c:ee", c.TapMACAddr)
	assert.Equal(t, "wsltap", c.TapName)
	assert.Equal(t, "/etc/wsl-vpnfix/wsl-gvproxy.exe", c.GvproxyPath)
	assert.Equal(t, "/sbin/wsl-gvforwarder", c.GvforwarderPath)
	assert.Equal(t, "example.com", c.CheckHost)
	assert.Equal(t, "1.1.1.1", c.CheckDNS)
	assert.False(t, c.Debug)
}
