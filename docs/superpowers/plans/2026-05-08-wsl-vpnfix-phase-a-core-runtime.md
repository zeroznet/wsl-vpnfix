<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# wsl-vpnfix Phase A — Core Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A working `wsl-vpnfix` Go binary that, when run as root inside WSL 2, validates config, creates the `wsltap` tap interface, spawns `gvforwarder` (which itself spawns `wsl-gvproxy.exe` on Windows via WSL interop), installs nftables NAT rules, runs healthchecks, and tears everything down cleanly on signal. Manually testable end-to-end on a developer's WSL 2 setup. No rootfs assembly, no CI, no signing — those land in Phase B and C.

**Architecture (verified against `containers/gvisor-tap-vsock` v0.8.8 source):**

The orchestrator spawns **one** child: `gvforwarder`. The forwarder spawns `wsl-gvproxy.exe` itself via the `stdio:` URL scheme (`pkg/transport/dial_linux.go:34-42`, `pkg/net/stdio/dial.go:11`). The orchestrator never spawns the .exe directly — there is no transport endpoint in v0.8.8 that accepts pre-existing fds, so manual `io.Pipe` wiring between two children cannot work.

Subsystems live in `internal/`:
- `config` — env parsing + strict validation
- `netlink` — tap, addr, route via `vishvananda/netlink`
- `netfilter` — nftables NAT rule construction + install via `google/nftables`
- `process` — child spawn, env-allowlist, signal-aware termination
- `wsl` — WSL2 NAT gateway IP detection (resolv.conf parsing with WSL marker check)
- `healthcheck` — post-start probes (HTTP GET, DNS resolve)

`cmd/wsl-vpnfix/main.go` is a thin wiring layer.

**Tech stack:**
- Go 1.25+ (`CGO_ENABLED=0`, single static binary)
- `github.com/vishvananda/netlink` for tap/addr/route ops
- `github.com/google/nftables` for NAT rule construction
- Standard library for process management
- `testing` + `github.com/stretchr/testify` for tests

**Out of scope for Phase A:** rootfs (Alpine, Dockerfile, pack.sh), reproducible build, GitHub Actions CI, cosign signing, SBOM, install-wslvpnfix.ps1, README, audit doc, threat model doc. Those come in Phase B / C.

---

## File Structure

Files created in this phase:

```
wsl-vpnfix/
├── .gitignore
├── go.mod
├── go.sum
├── cmd/
│   └── wsl-vpnfix/
│       ├── main.go                       ← wiring + flag parsing
│       └── main_test.go                  ← unit tests for buildEnv allowlist
├── internal/
│   ├── config/
│   │   ├── config.go                     ← Config struct + Load() from env
│   │   ├── config_test.go                ← unit tests
│   │   ├── validators.go                 ← regex validators for IP, MAC, path, hostname
│   │   └── validators_test.go            ← unit tests
│   ├── netlink/
│   │   ├── tap.go                        ← tap create / up / down / del / route
│   │   └── tap_test.go                   ← integration tests (root + CAP_NET_ADMIN)
│   ├── netfilter/
│   │   ├── rules.go                      ← nft rule construction + install/remove
│   │   └── rules_test.go                 ← unit tests on rule shape; integration for install
│   ├── process/
│   │   ├── manager.go                    ← Manager: spawn, wait, reap, signal-aware
│   │   └── manager_test.go               ← unit tests with /bin/true and /bin/sleep
│   ├── wsl/
│   │   ├── resolvconf.go                 ← WSL2 NAT gateway IP from resolv.conf
│   │   └── resolvconf_test.go            ← unit tests (marker-check + parsing)
│   └── healthcheck/
│       ├── checks.go                     ← HTTP GET and DNS resolve probes
│       └── checks_test.go                ← unit tests
```

**Boundaries:**
- `internal/config` only knows about env vars and types. No syscalls.
- `internal/netlink` only knows about tap/addr/route. No process spawning.
- `internal/netfilter` only knows about nft rule construction and idempotent install/remove. No networking checks.
- `internal/process` is generic child management; doesn't know about gvforwarder specifically.
- `internal/wsl` is the WSL2 NAT gateway IP detector (resolv.conf parsing with WSL marker check). No process spawning.
- `internal/healthcheck` is pure I/O probing.
- `cmd/wsl-vpnfix/main.go` is the only place where these are wired together.

This is the unit-of-audit layout: every package has a narrow, testable surface.

---

## Task 1: Repo scaffold

**Files:**
- Create: `.gitignore`
- Create: `go.mod`
- Create: directories `cmd/wsl-vpnfix/`, `internal/config/`, `internal/netlink/`, `internal/netfilter/`, `internal/process/`, `internal/wsl/`, `internal/healthcheck/`
- Run: `git init`

- [ ] **Step 1: Initialize git**

```bash
cd /home/zero/dev/wsl-vpnfix
git init -b main
```

Expected: `Initialized empty Git repository in /home/zero/dev/wsl-vpnfix/.git/`

- [ ] **Step 2: Create `.gitignore`**

Path: `.gitignore`

```
# Binaries
/bin/
/out/
/wsl-vpnfix
*.exe

# Test artifacts
/coverage.out
/coverage.html

# Editor / OS
.vscode/
.idea/
.DS_Store
*.swp
```

`/wsl-vpnfix` is **root-anchored** with the leading slash on purpose. A bare `wsl-vpnfix` would also match the `cmd/wsl-vpnfix/` directory and silently swallow every untracked file inside it (e.g. a `main_test.go` added later). Found the hard way during Phase A.

- [ ] **Step 3: Create empty package directories**

```bash
mkdir -p cmd/wsl-vpnfix \
         internal/config \
         internal/netlink \
         internal/netfilter \
         internal/process \
         internal/wsl \
         internal/healthcheck
```

- [ ] **Step 4: Initialize Go module**

```bash
go mod init github.com/zeroznet/wsl-vpnfix
```

Expected: creates `go.mod` with module path and `go 1.25` (or current).

- [ ] **Step 5: Verify**

```bash
go vet ./... 2>&1 || true
git status
```

Expected: no compile errors (no Go files yet); `git status` shows untracked files.

- [ ] **Step 6: Commit**

```bash
git add .gitignore go.mod
git commit -m "scaffold: init repo, go module, gitignore"
```

---

## Task 2: Config struct and defaults

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Path: `internal/config/config_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

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
```

- [ ] **Step 2: Add testify to go.mod and run test (expect compile failure)**

```bash
go get github.com/stretchr/testify@latest
go test ./internal/config/... 2>&1
```

Expected: FAIL — `undefined: Default`.

- [ ] **Step 3: Implement minimal Config + Default()**

Path: `internal/config/config.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

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
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/config/... -v
```

Expected: `--- PASS: TestDefaultConfig_Values`.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go go.mod go.sum
git commit -m "config: add Config struct and Default() with upstream-compatible defaults"
```

---

## Task 3: Config validators

**Files:**
- Create: `internal/config/validators.go`
- Create: `internal/config/validators_test.go`

Strict regex per input class. Path validation rejects traversal (`..`) — defense in depth even though config inputs come from a trusted systemd unit.

- [ ] **Step 1: Write the failing tests**

Path: `internal/config/validators_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

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
		{"abcdefghijklmno", true},  // 15 chars, max
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
		{"/etc/../etc/shadow", false},   // traversal
		{"/etc/wsl-vpnfix/..", false},   // trailing traversal
		{"/..", false},
		{"/etc/./wsl-vpnfix", false},    // current-dir segment
		{"/-rf", false},                 // would smuggle as argv flag
		{"/-foo/bar", false},            // leading dash on first segment
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
```

- [ ] **Step 2: Run tests (expect compile failure)**

```bash
go test ./internal/config/... 2>&1
```

Expected: FAIL — `undefined: ValidateIPv4` etc.

- [ ] **Step 3: Implement validators**

Path: `internal/config/validators.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ipv4Re     = regexp.MustCompile(`^((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\.){3}(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)$`)
	macRe      = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)
	ifNameRe   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_]{0,14}$`)
	absPathRe  = regexp.MustCompile(`^/[A-Za-z0-9_][A-Za-z0-9_./-]*$`)
	hostnameRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)
)

func ValidateIPv4(s string) error {
	if !ipv4Re.MatchString(s) {
		return fmt.Errorf("invalid IPv4 address: %q", s)
	}
	return nil
}

func ValidateMAC(s string) error {
	if !macRe.MatchString(s) {
		return fmt.Errorf("invalid MAC address (expected colon-separated): %q", s)
	}
	return nil
}

func ValidateInterfaceName(s string) error {
	if !ifNameRe.MatchString(s) {
		return fmt.Errorf("invalid interface name (must match Linux IFNAMSIZ rules): %q", s)
	}
	return nil
}

// ValidateAbsolutePath rejects relative paths, paths with shell metacharacters,
// and paths containing `..` or `.` segments. The latter blocks traversal even
// if the caller forgets to filepath.Clean before use.
func ValidateAbsolutePath(s string) error {
	if !absPathRe.MatchString(s) {
		return fmt.Errorf("invalid absolute path: %q", s)
	}
	cleaned := filepath.Clean(s)
	if cleaned != s {
		return fmt.Errorf("path must be in cleaned form (no `.` or `..` segments): %q", s)
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." || seg == "." {
			return fmt.Errorf("path contains traversal segment: %q", s)
		}
	}
	return nil
}

func ValidateHostname(s string) error {
	if !hostnameRe.MatchString(s) {
		return fmt.Errorf("invalid hostname: %q", s)
	}
	return nil
}
```

Note: `ifNameRe` drops the `.` allowed in upstream; Linux kernel rejects `.` and `..` as link names anyway, and dropping the dot from the regex is one less special case to reason about.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/config/... -v
```

Expected: every subtest PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/validators.go internal/config/validators_test.go
git commit -m "config: add strict validators for IP, MAC, ifname, path, hostname (path traversal rejected)"
```

---

## Task 4: Env loader wired with validators

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Add tests for `Load()`**

Append to `internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Run tests (expect failure)**

```bash
go test ./internal/config/... -run TestLoad -v
```

Expected: FAIL — `undefined: Load`.

- [ ] **Step 3: Implement `Load()`**

Append to `internal/config/config.go`:

```go
import (
	"fmt"
	"os"
)

// Load builds a Config by starting from Default() and applying
// any non-empty matching environment variables. Every override
// is validated; any invalid value returns an error and aborts.
func Load() (Config, error) {
	c := Default()

	type field struct {
		envName  string
		dst      *string
		validate func(string) error
	}
	stringFields := []field{
		{"WSL2_GATEWAY_IP", &c.WSL2GatewayIP, ValidateIPv4},
		{"TAP_MAC_ADDR", &c.TapMACAddr, ValidateMAC},
		{"TAP_NAME", &c.TapName, ValidateInterfaceName},
		{"GVPROXY_PATH", &c.GvproxyPath, ValidateAbsolutePath},
		{"GVFORWARDER_PATH", &c.GvforwarderPath, ValidateAbsolutePath},
		{"CHECK_HOST", &c.CheckHost, ValidateHostname},
		{"CHECK_DNS", &c.CheckDNS, ValidateIPv4},
	}
	for _, f := range stringFields {
		v := os.Getenv(f.envName)
		if v == "" {
			continue
		}
		if err := f.validate(v); err != nil {
			return Config{}, fmt.Errorf("env %s: %w", f.envName, err)
		}
		*f.dst = v
	}

	switch os.Getenv("DEBUG") {
	case "":
		// keep default
	case "0", "false", "FALSE", "False":
		c.Debug = false
	case "1", "true", "TRUE", "True":
		c.Debug = true
	default:
		return Config{}, fmt.Errorf("env DEBUG: invalid bool value %q", os.Getenv("DEBUG"))
	}

	return c, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/config/... -v
```

Expected: every test PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "config: load from env with per-field validation"
```

---

## Task 5: Netlink — tap creation and lifecycle

**Files:**
- Create: `internal/netlink/tap.go`
- Create: `internal/netlink/tap_test.go`

Real tap creation requires `CAP_NET_ADMIN`. Tests are integration-tagged and skip when not root. Error matching uses `errors.Is(err, unix.EEXIST)` rather than string compare.

- [ ] **Step 1: Add netlink dependency**

```bash
go get github.com/vishvananda/netlink@latest
```

- [ ] **Step 2: Write the integration test**

Path: `internal/netlink/tap_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

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
```

- [ ] **Step 3: Run integration tests (expect compile failure)**

```bash
go test -tags=integration ./internal/netlink/... 2>&1
```

Expected: compile error — `undefined: CreateTap` etc.

- [ ] **Step 4: Implement `internal/netlink/tap.go`**

Path: `internal/netlink/tap.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

// Package netlink wraps github.com/vishvananda/netlink with the
// narrow set of operations wsl-vpnfix needs: tap create / address /
// up / down / delete, and default-route install.
package netlink

import (
	"errors"
	"fmt"
	"net"

	vnl "github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// CreateTap creates a TUN/TAP device of TAP type with the given name
// and MAC address. Idempotency: returns nil if a tap with the same
// name already exists with the same MAC; returns an error if the name
// exists but the type or MAC differs.
func CreateTap(name, mac string) error {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("parse MAC: %w", err)
	}

	if existing, err := vnl.LinkByName(name); err == nil {
		if _, ok := existing.(*vnl.Tuntap); !ok {
			return fmt.Errorf("link %q exists but is not a tap device", name)
		}
		if existing.Attrs().HardwareAddr.String() != hw.String() {
			return fmt.Errorf("tap %q exists with different MAC %s", name, existing.Attrs().HardwareAddr)
		}
		return nil
	}

	la := vnl.NewLinkAttrs()
	la.Name = name
	la.HardwareAddr = hw
	tap := &vnl.Tuntap{LinkAttrs: la, Mode: vnl.TUNTAP_MODE_TAP}
	if err := vnl.LinkAdd(tap); err != nil {
		return fmt.Errorf("link add: %w", err)
	}
	// vishvananda/netlink's *Tuntap path uses TUNSETIFF, which only sets
	// name+mode; LinkAttrs.HardwareAddr is ignored. The kernel auto-assigns
	// a random MAC, so we must set ours explicitly. Mirrors upstream's
	// `ip link set dev $TAP_NAME address $TAP_MAC_ADDR`.
	link, err := vnl.LinkByName(name)
	if err != nil {
		return fmt.Errorf("link by name (post-add): %w", err)
	}
	if err := vnl.LinkSetHardwareAddr(link, hw); err != nil {
		return fmt.Errorf("set hardware addr: %w", err)
	}
	return nil
}

// AddAddr assigns IPv4 addr/prefixLen to the link. Idempotent.
func AddAddr(name, addr string, prefixLen int) error {
	link, err := vnl.LinkByName(name)
	if err != nil {
		return fmt.Errorf("link by name: %w", err)
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return fmt.Errorf("parse addr: %q", addr)
	}
	a := &vnl.Addr{IPNet: &net.IPNet{IP: ip, Mask: net.CIDRMask(prefixLen, 32)}}
	if err := vnl.AddrAdd(link, a); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return nil
		}
		return fmt.Errorf("addr add: %w", err)
	}
	return nil
}

func SetUp(name string) error {
	link, err := vnl.LinkByName(name)
	if err != nil {
		return fmt.Errorf("link by name: %w", err)
	}
	return vnl.LinkSetUp(link)
}

func SetDown(name string) error {
	link, err := vnl.LinkByName(name)
	if err != nil {
		if isLinkNotFound(err) {
			return nil
		}
		return fmt.Errorf("link by name: %w", err)
	}
	return vnl.LinkSetDown(link)
}

func DeleteTap(name string) error {
	link, err := vnl.LinkByName(name)
	if err != nil {
		if isLinkNotFound(err) {
			return nil
		}
		return fmt.Errorf("link by name: %w", err)
	}
	return vnl.LinkDel(link)
}

func isLinkNotFound(err error) bool {
	if err == nil {
		return false
	}
	var lnf vnl.LinkNotFoundError
	return errors.As(err, &lnf)
}
```

- [ ] **Step 5: Run integration tests**

```bash
go test -tags=integration ./internal/netlink/... -v                  # skips on dev box
sudo -E env "PATH=$PATH" go test -tags=integration ./internal/netlink/... -v
```

Expected: skip without sudo; full PASS with sudo.

- [ ] **Step 6: Commit**

```bash
git add internal/netlink/tap.go internal/netlink/tap_test.go go.mod go.sum
git commit -m "netlink: tap create/up/down/del + addr add (idempotent, errno-typed)"
```

---

## Task 6: Netlink — default route via tap

**Files:**
- Modify: `internal/netlink/tap.go`
- Modify: `internal/netlink/tap_test.go`

- [ ] **Step 1: Append integration test**

Append to `internal/netlink/tap_test.go`:

```go
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
```

`isDefaultDst` (defined in `tap.go`, see step 3) accepts both representations of an IPv4 default. Using a bare `r.Dst == nil` check would miss any kernel that surfaces the route as `Dst=0.0.0.0/0` (which is what the dev-container kernel does today).

- [ ] **Step 2: Run test (expect compile failure)**

```bash
go test -tags=integration ./internal/netlink/... -run TestRoute -v
```

Expected: FAIL — `undefined: AddDefaultRoute`.

- [ ] **Step 3: Implement `AddDefaultRoute`, `DelDefaultRoute`**

Append to `internal/netlink/tap.go`:

```go
// AddDefaultRoute installs an IPv4 default route via gateway out of link.
// Idempotent: returns nil if the route is already present.
func AddDefaultRoute(linkName, gateway string) error {
	link, err := vnl.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("link by name: %w", err)
	}
	gw := net.ParseIP(gateway)
	if gw == nil {
		return fmt.Errorf("parse gateway: %q", gateway)
	}
	r := &vnl.Route{
		LinkIndex: link.Attrs().Index,
		Gw:        gw,
		Dst:       nil,
	}
	if err := vnl.RouteAdd(r); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return nil
		}
		return fmt.Errorf("route add: %w", err)
	}
	return nil
}

// DelDefaultRoute removes the IPv4 default route via gateway out of link
// if present. Returns nil if the route or link is gone.
func DelDefaultRoute(linkName, gateway string) error {
	link, err := vnl.LinkByName(linkName)
	if err != nil {
		if isLinkNotFound(err) {
			return nil
		}
		return fmt.Errorf("link by name: %w", err)
	}
	gw := net.ParseIP(gateway)
	if gw == nil {
		return fmt.Errorf("parse gateway: %q", gateway)
	}
	r := &vnl.Route{
		LinkIndex: link.Attrs().Index,
		Gw:        gw,
		Dst:       nil,
	}
	if err := vnl.RouteDel(r); err != nil {
		if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) {
			return nil
		}
		return fmt.Errorf("route del: %w", err)
	}
	return nil
}

// RouteSnapshot is an opaque handle to a set of captured routes. Treat
// the value as read-only and pass it back to RestoreRoutes. The internal
// representation is deliberately hidden so that swapping the underlying
// netlink library (open item in spec section 8) is a one-package change
// — callers never bind to vishvananda/netlink types through this API.
type RouteSnapshot struct {
	routes []vnl.Route
}

// Len reports the number of captured routes. Callers can use this to
// skip pushing a teardown closure when there is nothing to undo.
func (s RouteSnapshot) Len() int { return len(s.routes) }

// isDefaultDst reports whether dst represents an IPv4 default route.
// vnl.RouteList may return either nil OR an explicit *net.IPNet with a
// /0 mask depending on the kernel version (and the rtnetlink path that
// produced the entry). Both mean the same thing; treat them the same.
// A bare `r.Dst == nil` check would silently miss every default route
// surfaced as 0.0.0.0/0 — exactly the case CaptureAndDelDefaultRoutes
// must NOT miss, since leaving a stale default in place defeats the
// whole point of redirecting WSL2 traffic through our tap.
func isDefaultDst(dst *net.IPNet) bool {
	if dst == nil {
		return true
	}
	ones, bits := dst.Mask.Size()
	return ones == 0 && bits == 32
}

// CaptureAndDelDefaultRoutes lists all IPv4 default routes in the main
// table, deletes them, and returns an opaque snapshot the caller can
// hand back to RestoreRoutes on teardown. Used at startup to clear the
// WSL2 NAT default before installing the wsl-vpnfix tap default —
// without losing the user's actual route topology if it differs from
// the stock `eth0` shape.
func CaptureAndDelDefaultRoutes() (RouteSnapshot, error) {
	routes, err := vnl.RouteList(nil, vnl.FAMILY_V4)
	if err != nil {
		return RouteSnapshot{}, fmt.Errorf("list routes: %w", err)
	}
	var captured []vnl.Route
	for _, r := range routes {
		if !isDefaultDst(r.Dst) {
			continue
		}
		c := r // copy by value before storing/deleting
		captured = append(captured, c)
	}
	for i := range captured {
		if err := vnl.RouteDel(&captured[i]); err != nil {
			if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) {
				continue
			}
			return RouteSnapshot{routes: captured}, fmt.Errorf("route del default via %s: %w", captured[i].Gw, err)
		}
	}
	return RouteSnapshot{routes: captured}, nil
}

// RestoreRoutes re-installs each captured route. Idempotent: skips
// routes already present (EEXIST). An empty snapshot is a no-op. Used
// by orchestrator teardown to undo CaptureAndDelDefaultRoutes.
func RestoreRoutes(s RouteSnapshot) error {
	for i := range s.routes {
		if err := vnl.RouteAdd(&s.routes[i]); err != nil {
			if errors.Is(err, unix.EEXIST) {
				continue
			}
			return fmt.Errorf("route restore via %s: %w", s.routes[i].Gw, err)
		}
	}
	return nil
}
```

`CaptureAndDelDefaultRoutes` mirrors upstream `wsl2tap_down`'s `ip route del default` step but captures the original `(LinkIndex, Gw, ...)` tuples instead of forgetting them. The result is returned as an opaque `RouteSnapshot` value — `vishvananda/netlink` types do not escape the package — so a future swap to `mdlayher/netlink` (spec section 8) does not touch `cmd/wsl-vpnfix/main.go`. The orchestrator stashes the snapshot in a teardown closure that calls `RestoreRoutes`, so a user with policy routing or a non-`eth0` default is restored to their actual prior state — not to a hardcoded best-effort guess.

- [ ] **Step 4: Run integration test with root**

```bash
sudo -E env "PATH=$PATH" go test -tags=integration ./internal/netlink/... -run TestRoute -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/netlink/tap.go internal/netlink/tap_test.go
git commit -m "netlink: add/del IPv4 default route via tap, plus capture/restore helpers for the original WSL2 default"
```

---

## Task 7: nftables — rule construction (unit-tested)

**Files:**
- Create: `internal/netfilter/rules.go`
- Create: `internal/netfilter/rules_test.go`

The rule shape is built in pure Go and unit-tested before any kernel install. `BuildRuleSet` returns `(RuleSet, error)` — no panics.

- [ ] **Step 1: Add nftables dependency**

```bash
go get github.com/google/nftables@latest
```

- [ ] **Step 2: Write the unit test**

Path: `internal/netfilter/rules_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

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
```

- [ ] **Step 3: Run test (expect compile failure)**

```bash
go test ./internal/netfilter/... 2>&1
```

Expected: FAIL — `undefined: BuildRuleSet, Params, RuleSet`.

- [ ] **Step 4: Implement `internal/netfilter/rules.go`**

Path: `internal/netfilter/rules.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

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
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/netfilter/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/netfilter/rules.go internal/netfilter/rules_test.go go.mod go.sum
git commit -m "netfilter: pure-Go RuleSet builder, error-return on missing inputs"
```

---

## Task 8: nftables — install and remove

**Files:**
- Modify: `internal/netfilter/rules.go`
- Modify: `internal/netfilter/rules_test.go` (split: keep unit test in `rules_test.go`, add integration test in `rules_install_test.go`)

The Install / Remove path is integration-tested only (requires CAP_NET_ADMIN). Unit tests stay separate from integration to keep `go test ./...` runnable on a dev box.

- [ ] **Step 1: Create separate integration test file**

Path: `internal/netfilter/rules_install_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

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
```

- [ ] **Step 2: Run test (expect compile failure)**

```bash
go test -tags=integration ./internal/netfilter/... 2>&1
```

Expected: FAIL — `undefined: Install, Remove`.

- [ ] **Step 3: Implement `Install` and `Remove`**

Append to `internal/netfilter/rules.go`:

```go
import (
	"errors"
	"net"

	nft "github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

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
```

(Merge the new `import` block with the existing one in `rules.go`.)

- [ ] **Step 4: Run tests with root**

```bash
sudo -E env "PATH=$PATH" go test -tags=integration ./internal/netfilter/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/netfilter/rules.go internal/netfilter/rules_install_test.go go.mod go.sum
git commit -m "netfilter: install/remove ruleset via netlink-typed nftables (idempotent)"
```

---

## Task 9: Process manager

**Files:**
- Create: `internal/process/manager.go`
- Create: `internal/process/manager_test.go`

`Spec.Env` follows Go's standard contract: `nil` inherits the parent env, `[]string{}` is empty. Both are valid; the caller chooses.

- [ ] **Step 1: Write the unit test**

Path: `internal/process/manager_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpawn_TrueExitsZero(t *testing.T) {
	m := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := m.Spawn(ctx, Spec{Path: "/bin/true"})
	require.NoError(t, err)

	err = h.Wait()
	assert.NoError(t, err)
}

func TestSpawn_FalseExitsNonZero(t *testing.T) {
	m := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := m.Spawn(ctx, Spec{Path: "/bin/false"})
	require.NoError(t, err)

	err = h.Wait()
	var exitErr *exec.ExitError
	assert.True(t, errors.As(err, &exitErr))
	assert.Equal(t, 1, exitErr.ExitCode())
}

func TestSpawn_RejectsRelativePath(t *testing.T) {
	m := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := m.Spawn(ctx, Spec{Path: "true"})
	assert.Error(t, err)
}

func TestSpawn_TerminatesOnContextCancel(t *testing.T) {
	if os.Getenv("CI_SKIP_SLEEP") == "1" {
		t.Skip("skipped under CI_SKIP_SLEEP")
	}
	m := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	h, err := m.Spawn(ctx, Spec{Path: "/bin/sleep", Args: []string{"30"}})
	require.NoError(t, err)

	err = h.Wait()
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || isSignaledKill(err), "got: %v", err)
}

// TestSpawn_TerminatesProcessGroup confirms that cancelling the context
// signals the entire process group, not just the leader pid. gvforwarder
// spawns wsl-gvproxy.exe without its own Setpgid, so a leader-only signal
// would orphan the .exe.
func TestSpawn_TerminatesProcessGroup(t *testing.T) {
	if os.Getenv("CI_SKIP_SLEEP") == "1" {
		t.Skip("skipped under CI_SKIP_SLEEP")
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	m := NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parent shell backgrounds /bin/sleep, records its pid, waits.
	// If we signal only the leader (the shell), the grandchild lives.
	// If we signal the pgroup, both die.
	h, err := m.Spawn(ctx, Spec{
		Path: "/bin/sh",
		Args: []string{"-c", "/bin/sleep 30 & echo $! > " + pidFile + "; wait"},
	})
	require.NoError(t, err)

	var grandPid int
	require.Eventually(t, func() bool {
		b, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		s := strings.TrimSpace(string(b))
		if s == "" {
			return false
		}
		p, err := strconv.Atoi(s)
		if err != nil {
			return false
		}
		grandPid = p
		return true
	}, 2*time.Second, 20*time.Millisecond, "grandchild pid file never appeared")

	cancel()
	_ = h.Wait()

	// kernel needs a tick to reap.
	time.Sleep(200 * time.Millisecond)

	// On a fully reaped grandchild, kill(pid, 0) returns ESRCH. In a pid
	// namespace whose PID 1 does not reap orphans (rootless podman dev
	// container), the SIGTERM still kills the process but it lingers as
	// a zombie until the namespace exits. Either outcome proves the
	// pgroup signal reached the grandchild; only "alive and Running /
	// Sleeping" is a real failure. On a real WSL 2 distro PID 1 is
	// systemd, which reaps and ESRCH wins.
	err = syscall.Kill(grandPid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return
	}
	require.NoError(t, err, "kill(pid, 0) returned unexpected error for pid %d", grandPid)
	statusBytes, statErr := os.ReadFile(fmt.Sprintf("/proc/%d/status", grandPid))
	require.NoError(t, statErr, "could not read /proc/%d/status to verify grandchild state", grandPid)
	assert.Contains(t, string(statusBytes), "State:\tZ",
		"grandchild pid %d is still alive (not zombie); pgroup signal failed:\n%s", grandPid, statusBytes)
}

func isSignaledKill(err error) bool {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return false
	}
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
		return ws.Signaled() && (ws.Signal() == syscall.SIGTERM || ws.Signal() == syscall.SIGKILL)
	}
	return false
}
```

- [ ] **Step 2: Run test (expect compile failure)**

```bash
go test ./internal/process/... 2>&1
```

Expected: FAIL — `undefined: NewManager, Spec, ...`.

- [ ] **Step 3: Implement `internal/process/manager.go`**

Path: `internal/process/manager.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

// Package process spawns child processes with a strict policy:
// absolute paths only, explicit env (nil = inherit, []string{} = empty),
// context-driven termination (SIGTERM then SIGKILL via WaitDelay).
package process

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// Spec describes a process to spawn.
type Spec struct {
	Path   string
	Args   []string
	Env    []string  // nil = inherit parent env (Go's default); []string{} = empty
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Handle wraps a running child.
type Handle struct {
	cmd  *exec.Cmd
	done chan error
}

type Manager struct{}

func NewManager() *Manager { return &Manager{} }

// Spawn starts the process and returns a Handle. The child receives SIGTERM
// when ctx is cancelled; if it does not exit within WaitDelay, SIGKILL.
// Path must be absolute.
func (m *Manager) Spawn(ctx context.Context, s Spec) (*Handle, error) {
	if !filepath.IsAbs(s.Path) {
		return nil, fmt.Errorf("process: Path must be absolute, got %q", s.Path)
	}

	cmd := exec.CommandContext(ctx, s.Path, s.Args...)
	cmd.Env = s.Env
	cmd.Stdin = s.Stdin
	cmd.Stdout = s.Stdout
	cmd.Stderr = s.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Signal the entire process group, not just the leader. gvforwarder
	// spawns wsl-gvproxy.exe (via WSL interop) without its own Setpgid, so
	// the .exe inherits this group but is not the leader. SIGTERM to the
	// pid alone would leave the .exe orphaned. `kill -pgid` mirrors upstream
	// wsl-vpnkit's `kill 0`.
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("process: start %s: %w", s.Path, err)
	}

	h := &Handle{cmd: cmd, done: make(chan error, 1)}
	go func() { h.done <- cmd.Wait() }()
	return h, nil
}

func (h *Handle) Wait() error          { return <-h.done }
func (h *Handle) Done() <-chan error   { return h.done }

func (h *Handle) Pid() int {
	if h.cmd == nil || h.cmd.Process == nil {
		return -1
	}
	return h.cmd.Process.Pid
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/process/... -v
```

Expected: all PASS (the cancel test exits in ~1s because /bin/sleep handles SIGTERM; only a child that ignores SIGTERM would consume the full 5s WaitDelay).

- [ ] **Step 5: Commit**

```bash
git add internal/process/manager.go internal/process/manager_test.go
git commit -m "process: spawn with absolute-path policy, SIGTERM-then-KILL on ctx cancel"
```

---

## Task 10: WSL2 NAT gateway IP detector

**Files:**
- Create: `internal/wsl/resolvconf.go`
- Create: `internal/wsl/resolvconf_test.go`

`internal/wsl` is a small package with one job: read the WSL2 NAT gateway IP from `/mnt/wsl/resolv.conf` (or `/etc/resolv.conf` as fallback). It does NOT spawn processes — gvforwarder is a Linux binary spawned via `internal/process`, and gvforwarder spawns `wsl-gvproxy.exe` itself via its `stdio:` URL scheme (verified against `containers/gvisor-tap-vsock` v0.8.8 source: `pkg/transport/dial_linux.go:34-42`).

The env allowlist (`buildEnv`) used to live in this package in an earlier draft as `BuildEnv` — generic env-allowlist code with no WSL specifics — but was inlined into `cmd/wsl-vpnfix/main.go` (Task 13) after the architectural diagnosis pass: it has exactly one caller and lives more honestly next to the spawn site that consumes it. The allowlist still names `WSL_INTEROP` explicitly, which is the load-bearing var for spawning `wsl-gvproxy.exe` via WSL's `binfmt_misc` interop (`microsoft/WSL` `src/linux/init/util.cpp:473-495`).

- [ ] **Step 1: Write the unit test**

Path: `internal/wsl/resolvconf_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

package wsl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWSL2GatewayIP_FromTempResolvConf(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "resolv.conf")
	require.NoError(t, os.WriteFile(p, []byte("# automatically generated by WSL\nnameserver 172.20.16.1\noptions edns0\n"), 0o644))

	got, err := wsl2GatewayIPFrom(p)
	require.NoError(t, err)
	assert.Equal(t, "172.20.16.1", got)
}

func TestWSL2GatewayIP_NoNameserver(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "resolv.conf")
	require.NoError(t, os.WriteFile(p, []byte("# automatically generated by WSL\n"), 0o644))

	_, err := wsl2GatewayIPFrom(p)
	assert.Error(t, err)
}

func TestWSL2GatewayIP_NoMarker_RejectsUserManagedFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "resolv.conf")
	// User-edited resolv.conf with a public resolver first — exactly the
	// case we must refuse, so we don't redirect 1.1.1.1 traffic by accident.
	require.NoError(t, os.WriteFile(p, []byte("nameserver 1.1.1.1\nnameserver 8.8.8.8\n"), 0o644))

	_, err := wsl2GatewayIPFrom(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marker absent")
}
```

- [ ] **Step 2: Run test (expect compile failure)**

```bash
go test ./internal/wsl/... 2>&1
```

Expected: FAIL — `undefined: wsl2GatewayIPFrom`.

- [ ] **Step 3: Implement `internal/wsl/resolvconf.go`**

Path: `internal/wsl/resolvconf.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

// Package wsl reads the WSL2 NAT gateway IP from resolv.conf with a
// safety check: only files carrying WSL's auto-generated marker comment
// are trusted. A user-edited resolv.conf with a public resolver first
// would otherwise silently misdirect our NAT redirect.
package wsl

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// WSL2GatewayIP returns the WSL2 NAT gateway IP. It reads
// /mnt/wsl/resolv.conf first (the single mount WSL exposes across all
// distros), falls back to /etc/resolv.conf. The first `nameserver` line
// is the gateway — but only if the file carries WSL's auto-generated
// marker comment. A user-edited resolv.conf may have a public resolver
// like 1.1.1.1 first, which would silently misdirect our NAT redirect;
// in that case we refuse and the caller must set WSL2_GATEWAY_IP
// explicitly.
func WSL2GatewayIP() (string, error) {
	var lastErr error
	for _, p := range []string{"/mnt/wsl/resolv.conf", "/etc/resolv.conf"} {
		ip, err := wsl2GatewayIPFrom(p)
		if err == nil {
			return ip, nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("autodetect WSL2 gateway: %w (set WSL2_GATEWAY_IP to override)", lastErr)
}

// autoGenMarker is the comment WSL writes at the top of resolv.conf when
// it manages the file. If it's absent, the file has been customized and
// we can't trust the first nameserver to be the WSL2 NAT gateway.
const autoGenMarker = "automatically generated by WSL"

func wsl2GatewayIPFrom(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	hasMarker := false
	var firstNS string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#") && strings.Contains(line, autoGenMarker) {
			hasMarker = true
			continue
		}
		if firstNS != "" {
			continue
		}
		if !strings.HasPrefix(line, "nameserver ") {
			continue
		}
		ns := strings.TrimSpace(strings.TrimPrefix(line, "nameserver "))
		if ns != "" {
			firstNS = ns
		}
	}
	if !hasMarker {
		return "", fmt.Errorf("%s: WSL auto-generated marker absent (file appears user-managed)", path)
	}
	if firstNS == "" {
		return "", fmt.Errorf("%s: no nameserver line found", path)
	}
	return firstNS, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/wsl/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wsl/resolvconf.go internal/wsl/resolvconf_test.go
git commit -m "wsl: WSL2 NAT gateway IP detector with auto-gen marker check"
```

---

## Task 11: Healthcheck probes

**Files:**
- Create: `internal/healthcheck/checks.go`
- Create: `internal/healthcheck/checks_test.go`

`ProbeHTTP` is the generic HTTP/HTTPS GET probe; the URL scheme determines whether TLS is used. Tests cover both an HTTP test server and a TLS test server.

- [ ] **Step 1: Write the unit test**

Path: `internal/healthcheck/checks_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProbeHTTP_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := ProbeHTTP(context.Background(), srv.URL, 2*time.Second, nil)
	assert.True(t, res.OK)
	assert.Equal(t, 200, res.StatusCode)
}

func TestProbeHTTP_TLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// httptest's TLS server uses a self-signed cert. The test server exposes
	// a Client() with that cert already trusted; we use it via the internal
	// probe entrypoint. Production callers pass nil tlsConf and rely on
	// system root CAs.
	res := probeHTTPWithClient(context.Background(), srv.URL, 2*time.Second, srv.Client())
	assert.True(t, res.OK)
	assert.Equal(t, 200, res.StatusCode)
}

func TestProbeHTTP_Timeout(t *testing.T) {
	res := ProbeHTTP(context.Background(), "http://127.0.0.1:1", 200*time.Millisecond, nil)
	assert.False(t, res.OK)
	assert.NotEmpty(t, res.Err)
}

func TestProbeDNS_ResolvesLocalhost(t *testing.T) {
	res := ProbeDNS(context.Background(), "localhost", "", 2*time.Second)
	assert.True(t, res.OK, "localhost must resolve via system resolver, got err: %s", res.Err)
}
```

- [ ] **Step 2: Run test (expect compile failure)**

```bash
go test ./internal/healthcheck/... 2>&1
```

Expected: FAIL — `undefined: ProbeHTTP, probeHTTPWithClient, ProbeDNS, Result`.

- [ ] **Step 3: Implement `internal/healthcheck/checks.go`**

Path: `internal/healthcheck/checks.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

// Package healthcheck runs informational connectivity probes after wsl-vpnfix
// finishes setup. Probes never gate startup; they are logged and returned.
package healthcheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Result of a single probe.
type Result struct {
	Name       string
	OK         bool
	StatusCode int
	Err        string
	Elapsed    time.Duration
}

// ProbeHTTP does a GET against url with the given timeout. Any URL scheme
// supported by net/http works (http://, https://). If tlsConf is non-nil it
// is used to override default TLS config — pass nil for system-roots TLS.
func ProbeHTTP(ctx context.Context, url string, timeout time.Duration, tlsConf *tls.Config) Result {
	cli := &http.Client{Timeout: timeout}
	if tlsConf != nil {
		cli.Transport = &http.Transport{TLSClientConfig: tlsConf}
	}
	return probeHTTPWithClient(ctx, url, timeout, cli)
}

func probeHTTPWithClient(ctx context.Context, url string, timeout time.Duration, cli *http.Client) Result {
	t0 := time.Now()
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(c, http.MethodGet, url, nil)
	if err != nil {
		return Result{Name: "http:" + url, OK: false, Err: err.Error(), Elapsed: time.Since(t0)}
	}
	resp, err := cli.Do(req)
	if err != nil {
		return Result{Name: "http:" + url, OK: false, Err: err.Error(), Elapsed: time.Since(t0)}
	}
	defer resp.Body.Close()
	return Result{
		Name:       "http:" + url,
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 400,
		StatusCode: resp.StatusCode,
		Elapsed:    time.Since(t0),
	}
}

// ProbeDNS resolves host using server (or system default if server == "").
func ProbeDNS(ctx context.Context, host, server string, timeout time.Duration) Result {
	t0 := time.Now()
	r := &net.Resolver{}
	if server != "" {
		r.PreferGo = true
		r.Dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		}
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	addrs, err := r.LookupHost(c, host)
	if err != nil {
		return Result{Name: fmt.Sprintf("dns:%s@%s", host, server), OK: false, Err: err.Error(), Elapsed: time.Since(t0)}
	}
	return Result{
		Name:    fmt.Sprintf("dns:%s@%s", host, server),
		OK:      len(addrs) > 0,
		Elapsed: time.Since(t0),
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/healthcheck/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/healthcheck/checks.go internal/healthcheck/checks_test.go
git commit -m "healthcheck: HTTP(S) GET and DNS resolve probes (informational)"
```

---

## Task 12: Main wiring — print-config flag

**Files:**
- Create: `cmd/wsl-vpnfix/main.go`

We start `main` simple: parse flags, load config, print or proceed.

- [ ] **Step 1: Implement minimal main with --print-config**

Path: `cmd/wsl-vpnfix/main.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

// Command wsl-vpnfix is the orchestrator for the wsl-vpnfix appliance distro.
// It validates config, sets up tap + nftables, spawns gvforwarder, runs
// healthchecks, and tears everything down on signal.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/zeroznet/wsl-vpnfix/internal/config"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	printConfig := flag.Bool("print-config", false, "print resolved config as JSON and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("wsl-vpnfix %s (%s)\n", version, commit)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %s\n", err)
		os.Exit(2)
	}

	if *printConfig {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "encode: %s\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintln(os.Stderr, "wsl-vpnfix: orchestrator wiring not yet implemented (Task 13)")
	os.Exit(1)
}
```

- [ ] **Step 2: Build and run --version**

```bash
go build -o /tmp/wsl-vpnfix ./cmd/wsl-vpnfix
/tmp/wsl-vpnfix --version
```

Expected: `wsl-vpnfix dev (none)`.

- [ ] **Step 3: Run --print-config**

```bash
/tmp/wsl-vpnfix --print-config
```

Expected: JSON dump of defaults.

- [ ] **Step 4: Run with invalid env**

```bash
WSL2_GATEWAY_IP=garbage /tmp/wsl-vpnfix --print-config
```

Expected: stderr `config: env WSL2_GATEWAY_IP: invalid IPv4 address: "garbage"`, exit code 2.

- [ ] **Step 5: Commit**

```bash
git add cmd/wsl-vpnfix/main.go
git commit -m "main: --version and --print-config flags; load+validate config"
```

---

## Task 13: Main wiring — full orchestrator lifecycle

**Files:**
- Modify: `cmd/wsl-vpnfix/main.go`

This is the critical wiring task. Highlights:

- **Single child:** the orchestrator spawns `gvforwarder` only. The forwarder spawns `wsl-gvproxy.exe` itself via `-url=stdio:.../wsl-gvproxy.exe?listen-stdio=accept&debug=N`. No manual stdio plumbing.
- **Forwarder flags:** `-stop-if-exist=""` (override the default that exits silently if `eth0` exists), `-preexisting=1` (orchestrator owns the tap), `-iface=$TAP_NAME`, `-debug=N`.
- **Env allowlist:** `WSL_INTEROP, PATH, WSL_DISTRO_NAME, WSLENV` are forwarded to gvforwarder. Empty `Env: []string{}` would break interop in non-trivial process trees.
- **Lifecycle is signal-driven, not child-exit-driven.** gvforwarder runs a forever-loop with 1s retry on error (`cmd/vm/main_linux.go:66-71`); if we wait for it to exit voluntarily we wait forever. Steady-state blocks on `signal.Notify(SIGINT, SIGTERM)`.
- **Teardown stack instead of `defer`.** Each setup step that creates kernel state appends a closure to a teardown slice; on signal we walk the slice in reverse and run every closure with bounded timeout. This survives signals received during init (which `defer` does not, because the closure isn't deferred until the line is reached).
- **WSL2 default route is captured + replaced.** Upstream's `wsl2tap_down` does `ip route del default` before installing the tap default. We do the same via `netlink.CaptureAndDelDefaultRoutes()`, but the routes come back as an opaque `netlink.RouteSnapshot` so teardown's `netlink.RestoreRoutes()` puts back the exact prior state — no hardcoded `eth0` fallback, and no `vishvananda/netlink` types escape into `cmd/`.
- **Phase decomposition.** `run()` is a thin dispatcher: install signal handler, then call six phase helpers in order (`resolveWSL2GatewayIP`, `captureDefaults`, `bringUpTap`, `installTapDefaultRoute`, `installNATRules`, `spawnGvforwarder`), then `startHealthchecks` + `waitForExit`. Each phase pushes its own teardown closure inside the helper, next to the operation that earned it. Order of phase calls in `run()` is the contract — swapping two phases would silently break partial-init recovery (e.g. nft install before tap up = `oifname` lookup against a nonexistent device).

- [ ] **Step 1: Replace main.go with the full orchestrator**

Path: `cmd/wsl-vpnfix/main.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

// Command wsl-vpnfix is the orchestrator for the wsl-vpnfix appliance distro.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zeroznet/wsl-vpnfix/internal/config"
	"github.com/zeroznet/wsl-vpnfix/internal/healthcheck"
	"github.com/zeroznet/wsl-vpnfix/internal/netfilter"
	"github.com/zeroznet/wsl-vpnfix/internal/netlink"
	"github.com/zeroznet/wsl-vpnfix/internal/process"
	"github.com/zeroznet/wsl-vpnfix/internal/wsl"
)

const (
	tableName        = "wsl-vpnfix"
	teardownStepWait = 10 * time.Second
)

var (
	version = "dev"
	commit  = "none"

	// envAllowlist is the explicit set of env vars copied to gvforwarder.
	// WSL_INTEROP: the path to the interop socket; required for spawning
	//   wsl-gvproxy.exe via WSL's binfmt_misc handler.
	// PATH: required by /init's lookup helpers and by the binfmt handler.
	// WSL_DISTRO_NAME: identifies the appliance distro to /init.
	// WSLENV: cross-OS env-propagation rules; preserves any user-set
	//   forwarding configured for the .exe (e.g. proxy/cert env).
	envAllowlist = []string{"WSL_INTEROP", "PATH", "WSL_DISTRO_NAME", "WSLENV"}
)

func main() {
	printConfig := flag.Bool("print-config", false, "print resolved config as JSON and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("wsl-vpnfix %s (%s)\n", version, commit)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fatal("config: %s", err)
	}

	if *printConfig {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cfg); err != nil {
			fatal("encode: %s", err)
		}
		return
	}

	if os.Geteuid() != 0 {
		fatal("must run as root (need CAP_NET_ADMIN for tap and nftables)")
	}

	if err := run(cfg); err != nil {
		fatal("%s", err)
	}
}

// teardown is an LIFO stack of cleanup closures. Each setup step appends to
// it on success; on signal or error we walk it in reverse and run every step
// with a bounded timeout. Survives signals received during init.
type teardown struct {
	mu    sync.Mutex
	steps []teardownStep
}

type teardownStep struct {
	name string
	fn   func() error
}

func (t *teardown) push(name string, fn func() error) {
	t.mu.Lock()
	t.steps = append(t.steps, teardownStep{name: name, fn: fn})
	t.mu.Unlock()
}

func (t *teardown) runAll() {
	t.mu.Lock()
	steps := append([]teardownStep(nil), t.steps...)
	t.steps = nil
	t.mu.Unlock()

	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		done := make(chan struct{})
		go func() {
			if err := s.fn(); err != nil {
				logf("teardown %s: %s", s.name, err)
			}
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(teardownStepWait):
			logf("teardown %s: timed out after %s", s.name, teardownStepWait)
		}
	}
}

// run wires the orchestrator: signal handling, six setup phases, child
// spawn, healthchecks, steady-state. The order of phase calls below is
// the contract — each phase's teardown closure is pushed inside the
// phase, so reordering would silently break partial-init recovery.
func run(cfg config.Config) error {
	td := &teardown{}
	defer td.runAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	installSignalHandler(cancel)

	wsl2GW, err := resolveWSL2GatewayIP(cfg)
	if err != nil {
		return err
	}

	if err := captureDefaults(td); err != nil {
		return err
	}
	if err := bringUpTap(td, cfg); err != nil {
		return err
	}
	if err := installTapDefaultRoute(td, cfg); err != nil {
		return err
	}
	if err := installNATRules(td, cfg, wsl2GW); err != nil {
		return err
	}

	fw, err := spawnGvforwarder(ctx, cfg)
	if err != nil {
		return err
	}

	startHealthchecks(ctx, cfg)
	waitForExit(ctx, cancel, fw)
	return nil
}

func installSignalHandler(cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sigs
		logf("signal %s, tearing down", s)
		cancel()
	}()
}

func resolveWSL2GatewayIP(cfg config.Config) (string, error) {
	if cfg.WSL2GatewayIP != "" {
		logf("WSL2 gateway IP (override via WSL2_GATEWAY_IP): %s", cfg.WSL2GatewayIP)
		return cfg.WSL2GatewayIP, nil
	}
	detected, err := wsl.WSL2GatewayIP()
	if err != nil {
		return "", fmt.Errorf("detect WSL2 gateway: %w", err)
	}
	logf("WSL2 gateway IP (autodetected): %s", detected)
	return detected, nil
}

// captureDefaults captures and removes existing IPv4 default routes so
// our tap default can be installed cleanly. The original routes go into
// a teardown closure that restores them verbatim — no hardcoded eth0
// fallback, so a user with policy routing or a non-default NIC name is
// restored to their actual prior state.
func captureDefaults(td *teardown) error {
	snap, err := netlink.CaptureAndDelDefaultRoutes()
	if err != nil {
		return fmt.Errorf("capture/clear existing default route: %w", err)
	}
	if snap.Len() > 0 {
		td.push("restore-original-default-routes", func() error {
			return netlink.RestoreRoutes(snap)
		})
	}
	return nil
}

func bringUpTap(td *teardown, cfg config.Config) error {
	if err := netlink.CreateTap(cfg.TapName, cfg.TapMACAddr); err != nil {
		return fmt.Errorf("tap create: %w", err)
	}
	td.push("delete-tap", func() error { return netlink.DeleteTap(cfg.TapName) })

	if err := netlink.AddAddr(cfg.TapName, config.VpnkitLocalIP, config.TapPrefixLen); err != nil {
		return fmt.Errorf("tap addr: %w", err)
	}
	if err := netlink.SetUp(cfg.TapName); err != nil {
		return fmt.Errorf("tap up: %w", err)
	}
	return nil
}

func installTapDefaultRoute(td *teardown, cfg config.Config) error {
	if err := netlink.AddDefaultRoute(cfg.TapName, config.VpnkitGatewayIP); err != nil {
		return fmt.Errorf("default route: %w", err)
	}
	td.push("delete-tap-default-route", func() error {
		return netlink.DelDefaultRoute(cfg.TapName, config.VpnkitGatewayIP)
	})
	return nil
}

func installNATRules(td *teardown, cfg config.Config, wsl2GW string) error {
	rs, err := netfilter.BuildRuleSet(netfilter.Params{
		WSL2GatewayIP:   wsl2GW,
		VpnkitGatewayIP: config.VpnkitGatewayIP,
		VpnkitHostIP:    config.VpnkitHostIP,
		VpnkitLocalCIDR: config.VpnkitLocalCIDR,
		TapName:         cfg.TapName,
	})
	if err != nil {
		return fmt.Errorf("nftables build: %w", err)
	}
	if err := netfilter.Install(rs, tableName); err != nil {
		return fmt.Errorf("nftables install: %w", err)
	}
	td.push("nftables-remove", func() error { return netfilter.Remove(tableName) })
	return nil
}

// spawnGvforwarder launches gvforwarder as our single child. The forwarder
// spawns wsl-gvproxy.exe itself via its `stdio:` URL scheme — we never
// spawn the .exe directly.
func spawnGvforwarder(ctx context.Context, cfg config.Config) (*process.Handle, error) {
	debugFlag := boolStr(cfg.Debug)
	stdioURL := fmt.Sprintf("stdio:%s?listen-stdio=accept&debug=%s", cfg.GvproxyPath, debugFlag)
	spec := process.Spec{
		Path: cfg.GvforwarderPath,
		Args: []string{
			"-url=" + stdioURL,
			"-iface=" + cfg.TapName,
			"-stop-if-exist=",
			"-preexisting=1",
			"-mac=" + cfg.TapMACAddr,
			"-debug=" + debugFlag,
		},
		Env:    buildEnv(envAllowlist),
		Stdout: os.Stderr,
		Stderr: os.Stderr,
	}
	logf("spawning gvforwarder: %s", strings.Join(spec.Args, " "))
	h, err := process.NewManager().Spawn(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("spawn gvforwarder: %w", err)
	}
	return h, nil
}

func startHealthchecks(ctx context.Context, cfg config.Config) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
		logf("health: %v", healthcheck.ProbeDNS(ctx, cfg.CheckHost, cfg.CheckDNS, 3*time.Second))
		logf("health: %v", healthcheck.ProbeHTTP(ctx, "https://"+cfg.CheckHost, 5*time.Second, nil))
	}()
}

// waitForExit blocks until the forwarder exits or a signal cancels ctx.
// gvforwarder runs a forever-loop, so a clean voluntary exit is unusual;
// treat any exit as fault and tear down.
func waitForExit(ctx context.Context, cancel context.CancelFunc, fw *process.Handle) {
	select {
	case err := <-fw.Done():
		logf("gvforwarder exited: %v (forever-loop should not exit voluntarily)", err)
	case <-ctx.Done():
		logf("ctx cancelled, waiting for forwarder to exit")
	}

	cancel()
	select {
	case <-fw.Done():
	case <-time.After(teardownStepWait):
		logf("gvforwarder did not exit within %s after cancel", teardownStepWait)
	}
}

// buildEnv constructs a child env block containing only those variables
// in `allowlist` that are set in the parent env. Returns nil for an
// empty allowlist; see process.Spec docs for nil vs empty Env semantics.
func buildEnv(allowlist []string) []string {
	if len(allowlist) == 0 {
		return nil
	}
	out := make([]string, 0, len(allowlist))
	for _, k := range allowlist {
		v, ok := os.LookupEnv(k)
		if !ok {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "wsl-vpnfix: "+format+"\n", args...)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "wsl-vpnfix: "+format+"\n", args...)
	os.Exit(1)
}
```

Note on the route restore: the orchestrator captures every IPv4 default route via `netlink.CaptureAndDelDefaultRoutes()` before installing the tap default, then restores the captured slice verbatim on teardown via `netlink.RestoreRoutes`. No hardcoded `eth0`, no guessing — whatever the user had (single eth0 default, multi-table policy routing, custom interface name), they get back exactly. If `len(originalDefaults) == 0` we skip the restore closure entirely (nothing to undo).

- [ ] **Step 2: Add `cmd/wsl-vpnfix/main_test.go` for `buildEnv`**

Path: `cmd/wsl-vpnfix/main_test.go`

```go
// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

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
```

Run:

```bash
go test ./cmd/wsl-vpnfix/... -v
```

Expected: all PASS.

- [ ] **Step 3: Compile**

```bash
go build -o /tmp/wsl-vpnfix ./cmd/wsl-vpnfix
```

Expected: clean build.

- [ ] **Step 4: Smoke-test --print-config still works**

```bash
/tmp/wsl-vpnfix --print-config
```

Expected: JSON config dump, exit 0.

- [ ] **Step 5: Verify run as non-root fails cleanly**

```bash
/tmp/wsl-vpnfix
```

Expected: stderr `wsl-vpnfix: must run as root (need CAP_NET_ADMIN for tap and nftables)`, exit 1.

- [ ] **Step 6: Commit**

```bash
git add cmd/wsl-vpnfix/main.go cmd/wsl-vpnfix/main_test.go
git commit -m "main: full orchestrator — single child (gvforwarder), teardown stack, signal-driven; inline buildEnv with tests"
```

---

## Task 14: End-to-end smoke test on a WSL host — **deferred to Phase B**

**Status:** out of scope for Phase A.

A binary-only smoke test that requires the operator to manually `apt install golang-go`, clone the repo, compile, and stage upstream binaries inside an arbitrary WSL distro — testing a flow no real user will ever follow — was rejected during pre-Phase-B review (2026-05-09). It would have validated a synthetic environment that diverges from production in exactly the surfaces (rootfs assembly, auto-start mechanism, `wsl.conf`, init wrapper) where bugs are most likely to land unnoticed.

The legitimate end-to-end gate is the **production tarball gate**, which lives in Phase B Task 1: build the importable `wsl-vpnfix-X.Y.tar.gz` rootfs from `build/Dockerfile.rootfs`, `wsl --import` it on a real WSL 2 machine with the operator's corporate VPN connected, and verify connectivity + clean teardown. Same outcome the synthetic smoke would have validated, against the artifact users actually receive.

Phase A's acceptance gate ends at Task 13: unit + integration tests green inside the dev container, race detector clean, reproducible build bit-for-bit identical. Those are the implementer-flow checkpoints. Production-shape smoke is Phase B's job.

---

## Self-Review

**Spec coverage (against `docs/superpowers/specs/2026-05-08-wsl-vpnfix-design.md`):**

| Spec section | Phase A task(s) | Phase later? |
|---|---|---|
| 1. Problem & target user | n/a (narrative only) | — |
| 2.1–2.3 Topology + components | Tasks 5–13 | — |
| 2.4 Lifecycle | Task 13 (full wiring, signal-driven) | — |
| 2.5 Configuration model | Tasks 2–4, 12 | — |
| 2.6 Rootfs contents | — | **Phase B** (rootfs assembly) |
| 3. Threat model | Tasks 2–4 (input validation, path traversal), 9 (process policy), 10 (env allowlist) | partial; full audit pass in **Phase C** |
| 4. Build & release pipeline | — | **Phase B** |
| 5. Repo layout — `cmd/`, `internal/` | Tasks 1–13 | — |
| 5. Repo layout — `build/`, `.github/`, `scripts/`, `docs/`, README, LICENSE | — | **Phase B / C** |
| 6. Decisions | reflected in code (Go, single child via stdio URL, nftables-via-netlink, amd64-only, Alpine-only) | — |
| 7. Out of scope | respected (no ARM, no library-of-gvisor-tap-vsock, no MS Store, no plugin, no Ubuntu/Fedora) | — |
| 8. Open items: nftables Go library | resolved → `github.com/google/nftables` | — |
| 8. Open items: netlink library | resolved → `github.com/vishvananda/netlink` | — |
| 8. Open items: PID 1 vs systemd | not yet resolved; binary works under either; revisit in **Phase B** | **Phase B** |
| 8. Open items: `wsl-vpnfixctl` debug subcommand | folded into the same binary as flags | — |

**Architecture corrections vs the original Phase A draft:**

The first draft of this plan assumed wsl-vpnfix would spawn `wsl-gvproxy.exe` directly via WSL's `binfmt_misc` interop and pipe stdio between it and gvforwarder manually. That model is incompatible with `containers/gvisor-tap-vsock` v0.8.8: gvforwarder calls `transport.Dial(endpoint)` which only accepts `vsock://`, `unix://`, or `stdio:` schemes — there is no transport that accepts pre-existing fds. The corrected model (single child = gvforwarder, .exe spawned by forwarder via the `stdio:` URL scheme) matches upstream wsl-vpnkit's shell architecture and is the only model that works with current upstream binaries. Verified against `pkg/transport/dial_linux.go:34-42`, `pkg/net/stdio/dial.go:11`, and `cmd/vm/main_linux.go:42-71`.

The first draft also passed `Env: []string{}` to the child, which would break WSL interop for any process tree shape that doesn't preserve a parent interop-socket descendant. The corrected model uses an explicit allowlist (`WSL_INTEROP`, `PATH`, `WSL_DISTRO_NAME`, `WSLENV`); see `microsoft/WSL` `src/linux/init/util.cpp:473-495` for the lookup logic.

**Code-review pass (2026-05-08) corrections:**

A pre-implementation review surfaced five concrete failures the plan would have shipped. All five were fixed in the plan, not deferred to code:

- **C-1 (Task 5)**: `vishvananda/netlink`'s `*Tuntap` path uses `TUNSETIFF`, which sets name+mode only; `LinkAttrs.HardwareAddr` is silently ignored, so the kernel auto-assigns a random MAC. `CreateTap` now calls `LinkSetHardwareAddr` after `LinkAdd`, mirroring upstream's `ip link set dev $TAP_NAME address $TAP_MAC_ADDR`.
- **C-2 (Task 9)**: `Setpgid: true` plus `cmd.Process.Signal(SIGTERM)` would orphan the gvforwarder process group on shutdown — `wsl-gvproxy.exe` (spawned by gvforwarder via interop) inherits the new pgroup but is not the leader. `cmd.Cancel` now signals `-pgid`, mirroring upstream wsl-vpnkit's `kill 0`. New `TestSpawn_TerminatesProcessGroup` confirms grandchild reaping.
- **C-3 (Tasks 6, 13)**: `DelExistingDefaultRoute` deleted every IPv4 default route and the teardown closure restored via hardcoded `eth0`, breaking any user with policy routing or a non-stock NIC. Replaced with `CaptureAndDelDefaultRoutes` (returns an opaque `RouteSnapshot` so `vishvananda/netlink` types do not leak through the package boundary) and `RestoreRoutes` (re-installs them verbatim). The orchestrator stashes the snapshot in a teardown closure with no eth0 fallback.
- **C-4 (Tasks 7, 8)**: The masquerade rule was scoped only by `oifname wsltap`, contradicting spec finding F-007 silently. Added `MatchSrcCIDR` to `Rule`, emit a saddr-mask check via `expr.Bitwise` + `expr.Cmp` against `Params.VpnkitLocalCIDR`. Test asserts the source-CIDR scope is present.
- **C-5 (Task 8)**: `Install` mixed `DelTable` + `AddTable` in one batched netlink transaction, which can abort under nft atomic semantics when the delete targets a non-existent table. Split into two `Conn`s: a delete batch that tolerates `ENOENT`, then a create batch that propagates real errors. New `TestInstall_ReplacesStaleTable` and `TestInstall_RejectsUnknownChain` cover the recovery paths.

Smaller corrections from the same pass: tightened `absPathRe` to forbid leading `-` (defense vs argv smuggling); added an `autoGenMarker` check + `WSL2_GATEWAY_IP` env override so a user-edited resolv.conf can't silently misdirect NAT; added a whitespace-only env-value rejection test; replaced a tautological DNS test with a real assertion; collapsed the `debugInt`/`boolStr` duplication; documented the env-allowlist rationale inline.

**Implementation-pass (2026-05-09) corrections:**

Three further failures surfaced during actual implementation against the dev-container kernel and were back-ported into the plan above. All reflect real production bugs the plan would have shipped:

- **C-6 (Task 6)**: Filtering default routes via `r.Dst == nil` is wrong: `vishvananda/netlink` may surface a default route as either `Dst=nil` OR `Dst=0.0.0.0/0` depending on kernel version and the rtnetlink path that produced the entry. The plan's bare `r.Dst == nil` check would silently miss every default route surfaced as 0.0.0.0/0, leaving the WSL2 NAT default in place at startup and defeating the whole redirect. Added an `isDefaultDst(*net.IPNet) bool` helper that accepts both forms; `CaptureAndDelDefaultRoutes` and the integration test both use it. Confirmed against Alpine 3.23.4 / Linux 6.12 in the dev container.
- **C-7 (Task 9)**: `TestSpawn_TerminatesProcessGroup` asserted `kill(grandPid, 0) == ESRCH` after pgroup signaling. That assertion fails inside any pid namespace whose PID 1 does not reap orphans (rootless podman containers, busybox-init Alpine, etc.) — the SIGTERM still kills the grandchild but it lingers as a zombie until the namespace exits, so `kill(0)` returns nil instead of ESRCH. Production code path (`Setpgid: true` + `cmd.Cancel = kill(-pid, SIGTERM)`) is correct; only the test was wrong. Fix: accept either ESRCH OR `State: Z` from `/proc/<pid>/status` as proof of pgroup-kill success. On real WSL 2 PID 1 is systemd, which reaps and ESRCH wins.
- **C-8 (Task 1)**: `.gitignore` had a bare `wsl-vpnfix` entry intended to ignore the built binary at the repo root. Without a leading slash it also matched the `cmd/wsl-vpnfix/` directory, silently swallowing every untracked file inside it (`main_test.go` was lost on first add). Anchored to `/wsl-vpnfix`.

**Placeholder scan:** searched for "TBD", "TODO", "implement later", "fill in details", "etc." — none in task content.

---

## After this plan

Phase A finishes at Task 13: unit + integration tests green inside the dev container, race detector clean, reproducible build bit-for-bit identical. The end-to-end gate (production tarball + `wsl --import` + connectivity verify on a real corporate-VPN WSL host) lives in Phase B Task 1 and is what actually closes the loop on Phase A's runtime.

- **Phase B plan** — rootfs assembly (Alpine pinned by digest, Dockerfile.builder, Dockerfile.rootfs, `build/upstream-pins.yaml`, `pack.sh`), reproducible build, GitHub Actions (`ci.yml`, `release.yml`, `reproducibility.yml`), cosign signing, syft SBOM, `wsl-vpnfix.service` with hardening directives.
- **Phase C plan** — README, LICENSE, `install-wslvpnfix.ps1`, `docs/SECURITY-AUDIT.md` with the initial audit pass, `docs/THREAT-MODEL.md` frozen from the spec, finding fixes round, tag `v1.0.0`.
