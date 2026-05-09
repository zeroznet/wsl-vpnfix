// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Sonnet 4.6)

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
