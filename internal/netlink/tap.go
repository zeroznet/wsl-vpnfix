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

// isDefaultDst reports whether dst represents a default (catch-all) route.
// vnl.RouteList may return either nil or the explicit 0.0.0.0/0 prefix
// depending on the kernel version; both represent the same default route.
func isDefaultDst(dst *net.IPNet) bool {
	if dst == nil {
		return true
	}
	ones, bits := dst.Mask.Size()
	return ones == 0 && bits == 32
}

func isLinkNotFound(err error) bool {
	if err == nil {
		return false
	}
	var lnf vnl.LinkNotFoundError
	return errors.As(err, &lnf)
}

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
