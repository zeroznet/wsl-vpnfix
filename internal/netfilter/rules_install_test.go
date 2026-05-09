// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

//go:build integration

package netfilter

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNotRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("integration test requires root / CAP_NET_ADMIN")
	}
}

func TestInstallRemove_RoundTrip(t *testing.T) {
	skipIfNotRoot(t)

	rs, err := BuildRuleSet(validParams())
	require.NoError(t, err)

	require.NoError(t, Install(rs, "wsl-vpnfix-test"))
	defer func() { _ = Remove("wsl-vpnfix-test") }()

	out, err := exec.Command("nft", "list", "table", "ip", "wsl-vpnfix-test").CombinedOutput()
	require.NoError(t, err, "nft list failed: %s", out)
	assert.True(t, strings.Contains(string(out), "prerouting"))
	assert.True(t, strings.Contains(string(out), "postrouting"))

	require.NoError(t, Remove("wsl-vpnfix-test"))

	_, err = exec.Command("nft", "list", "table", "ip", "wsl-vpnfix-test").CombinedOutput()
	assert.Error(t, err, "table should be gone after Remove")
}

func TestInstall_Idempotent(t *testing.T) {
	skipIfNotRoot(t)

	rs, err := BuildRuleSet(validParams())
	require.NoError(t, err)

	defer func() { _ = Remove("wsl-vpnfix-test-idem") }()

	require.NoError(t, Install(rs, "wsl-vpnfix-test-idem"))
	require.NoError(t, Install(rs, "wsl-vpnfix-test-idem"), "second install must succeed (replace semantics)")
}

// TestInstall_ReplacesStaleTable confirms Install handles the
// "previous run crashed mid-flight" case: a pre-existing table with
// content unrelated to ours must be cleanly replaced, not merged.
func TestInstall_ReplacesStaleTable(t *testing.T) {
	skipIfNotRoot(t)

	const tname = "wsl-vpnfix-stale"
	defer func() { _ = Remove(tname) }()

	// Pre-create a stale table with junk content via nft CLI.
	out, err := exec.Command("nft", "add", "table", "ip", tname).CombinedOutput()
	require.NoError(t, err, "pre-create stale table: %s", out)
	out, err = exec.Command("nft", "add", "chain", "ip", tname, "junk").CombinedOutput()
	require.NoError(t, err, "pre-create junk chain: %s", out)

	rs, err := BuildRuleSet(validParams())
	require.NoError(t, err)

	require.NoError(t, Install(rs, tname), "Install must replace a stale pre-existing table")

	out, err = exec.Command("nft", "list", "table", "ip", tname).CombinedOutput()
	require.NoError(t, err, "nft list failed: %s", out)
	listing := string(out)
	assert.True(t, strings.Contains(listing, "wsltap"), "new ruleset must be in place")
	assert.False(t, strings.Contains(listing, "junk"), "stale junk chain must be gone, got:\n%s", listing)
}

// TestInstall_RejectsUnknownChain confirms a malformed RuleSet does not
// touch kernel state (no partial install).
func TestInstall_RejectsUnknownChain(t *testing.T) {
	skipIfNotRoot(t)

	const tname = "wsl-vpnfix-bogus"
	defer func() { _ = Remove(tname) }()

	bad := RuleSet{Chains: []Chain{{Name: "not-a-real-chain"}}}
	err := Install(bad, tname)
	assert.Error(t, err)

	out, err := exec.Command("nft", "list", "table", "ip", tname).CombinedOutput()
	assert.Error(t, err, "table must not exist after rejected install, got: %s", out)
}
