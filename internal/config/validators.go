// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Haiku 4.5)

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
