// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Haiku 4.5)

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateIPv4(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"192.168.127.1", true},
		{"1.1.1.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"192.168.127", false},
		{"192.168.127.256", false},
		{"::1", false},
		{"abc", false},
		{"", false},
		{"192.168.127.1; rm -rf /", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			err := ValidateIPv4(c.in)
			if c.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestValidateMAC(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"5a:94:ef:e4:0c:ee", true},
		{"00:00:00:00:00:00", true},
		{"FF:FF:FF:FF:FF:FF", true},
		{"5a-94-ef-e4-0c-ee", false}, // we require colon form
		{"5a:94:ef:e4:0c", false},
		{"abc", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			err := ValidateMAC(c.in)
			if c.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestValidateInterfaceName(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"wsltap", true},
		{"eth0", true},
		{"a", true},
		{"abcdefghijklmno", true},   // 15 chars, max
		{"abcdefghijklmnop", false}, // 16 chars, too long
		{"with space", false},
		{"semi;colon", false},
		{"", false},
		{"-leading", false},
		{".", false},
		{"..", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			err := ValidateInterfaceName(c.in)
			if c.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestValidateAbsolutePath(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"/sbin/wsl-vpnfix", true},
		{"/etc/wsl-vpnfix/wsl-gvproxy.exe", true},
		{"sbin/wsl-vpnfix", false},
		{"./relative", false},
		{"", false},
		{"/with space/file", false},
		{"/with;semi", false},
		{"/with$dollar", false},
		{"/etc/../etc/shadow", false}, // traversal
		{"/etc/wsl-vpnfix/..", false}, // trailing traversal
		{"/..", false},
		{"/etc/./wsl-vpnfix", false}, // current-dir segment
		{"/-rf", false},              // would smuggle as argv flag
		{"/-foo/bar", false},         // leading dash on first segment
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			err := ValidateAbsolutePath(c.in)
			if c.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestValidateHostname(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"example.com", true},
		{"a.b.c", true},
		{"localhost", true},
		{"with space.com", false},
		{"with;semi", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			err := ValidateHostname(c.in)
			if c.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
