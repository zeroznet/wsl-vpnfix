// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildEnv_AllowlistedKept(t *testing.T) {
	t.Setenv("WSL_INTEROP", "/run/WSL/1_interop")
	t.Setenv("PATH", "/usr/local/sbin:/usr/sbin:/sbin")
	t.Setenv("WSL_DISTRO_NAME", "wsl-vpnfix")
	t.Setenv("RANDOM_SECRET", "must-not-leak")

	env := buildEnv([]string{"WSL_INTEROP", "PATH", "WSL_DISTRO_NAME"})

	assert.Contains(t, env, "WSL_INTEROP=/run/WSL/1_interop")
	assert.Contains(t, env, "PATH=/usr/local/sbin:/usr/sbin:/sbin")
	assert.Contains(t, env, "WSL_DISTRO_NAME=wsl-vpnfix")

	for _, kv := range env {
		assert.NotContains(t, kv, "RANDOM_SECRET", "non-allowlisted var leaked: %s", kv)
	}
}

func TestBuildEnv_EmptyAllowlist(t *testing.T) {
	t.Setenv("WSL_INTEROP", "/run/WSL/1_interop")
	env := buildEnv(nil)
	assert.Empty(t, env)
}

func TestBuildEnv_SkipsUnsetVars(t *testing.T) {
	os.Unsetenv("WSL_INTEROP")
	t.Setenv("PATH", "/usr/bin")
	env := buildEnv([]string{"WSL_INTEROP", "PATH"})
	assert.Contains(t, env, "PATH=/usr/bin")
	for _, kv := range env {
		assert.NotContains(t, kv, "WSL_INTEROP=", "missing var must be skipped, not produced as empty")
	}
}
