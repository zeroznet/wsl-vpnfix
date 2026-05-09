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

func TestLoad_NoEnv_ReturnsDefaults(t *testing.T) {
	t.Setenv("WSL2_GATEWAY_IP", "")
	t.Setenv("TAP_MAC_ADDR", "")
	t.Setenv("TAP_NAME", "")
	t.Setenv("GVPROXY_PATH", "")
	t.Setenv("GVFORWARDER_PATH", "")
	t.Setenv("CHECK_HOST", "")
	t.Setenv("CHECK_DNS", "")
	t.Setenv("DEBUG", "")

	got, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, Default(), got)
}

func TestLoad_OverridesValid(t *testing.T) {
	t.Setenv("WSL2_GATEWAY_IP", "10.0.0.1")
	t.Setenv("TAP_NAME", "tap1")
	t.Setenv("DEBUG", "1")

	got, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.1", got.WSL2GatewayIP)
	assert.Equal(t, "tap1", got.TapName)
	assert.True(t, got.Debug)
}

func TestLoad_RejectsInvalidIP(t *testing.T) {
	t.Setenv("WSL2_GATEWAY_IP", "not-an-ip")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WSL2_GATEWAY_IP")
}

func TestLoad_RejectsInjectionAttempt(t *testing.T) {
	t.Setenv("TAP_NAME", "wsltap; rm -rf /")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TAP_NAME")
}

func TestLoad_RejectsPathTraversal(t *testing.T) {
	t.Setenv("GVPROXY_PATH", "/etc/wsl-vpnfix/../../etc/passwd")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "GVPROXY_PATH")
}

func TestLoad_RejectsWhitespaceOnlyValue(t *testing.T) {
	t.Setenv("TAP_NAME", "   ")

	_, err := Load()
	assert.Error(t, err, "whitespace-only env value must not pass validation")
	assert.Contains(t, err.Error(), "TAP_NAME")
}

func TestLoad_WSL2GatewayOverride(t *testing.T) {
	t.Setenv("WSL2_GATEWAY_IP", "172.20.16.1")

	got, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "172.20.16.1", got.WSL2GatewayIP)
}
