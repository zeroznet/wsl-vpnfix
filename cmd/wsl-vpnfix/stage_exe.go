// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// gvproxyYAML is the minimal config file that works around an upstream
// regression in gvisor-tap-vsock v0.8.8: cmd/gvproxy/config.go:125
// declares the -listen-stdio CLI flag but the value is never written to
// config.Interfaces.Stdio (the "Patch config with CLI args" block at
// lines 224-262 covers qemu/bess/vfkit/vpnkit but skips stdio). Result:
// cmd/gvproxy/main.go:291's `if config.Interfaces.Stdio != ""` always
// fails, the AcceptStdio goroutine never starts, and the bridge is
// silently TX-only.
//
// Setting interfaces.stdio in YAML config bypasses the broken CLI wiring.
// Loading via -config also disables "default mode" (cmd/gvproxy/config.go:334),
// so we re-supply the static fields default mode would have populated:
// subnet, gateway IP, and gateway MAC. DHCP static lease and DNS zones
// are skipped because we statically assign 192.168.127.2 in the orchestrator
// and do not use containers.internal / docker.internal hostnames.
const gvproxyYAML = `interfaces:
  stdio: "stdio"
stack:
  subnet: "192.168.127.0/24"
  gatewayIP: "192.168.127.1"
  gatewayMacAddress: "5a:94:ef:e4:0c:dd"
  mtu: 1500
`

// stagedGvproxyDir is a Windows-native NTFS path (DrvFs, not 9P) writable
// to anyone WSL maps to. /mnt/c/Users/Public exists on every Windows install
// and inherits an "Authenticated Users: Modify" ACL by default.
const stagedGvproxyDir = "/mnt/c/Users/Public/.wsl-vpnfix"

// stageGvproxyExe copies the bundled gvproxy.exe from its Linux rootfs path
// (read by Windows via 9P) to a Windows-native NTFS path under DrvFs. The
// returned path is what gvforwarder must spawn from.
//
// Why: when WSL exec's a Windows .exe at a Linux 9P-backed path, Windows
// demand-pages the binary's code over 9P. On Windows 11 24H2 (build 26100)
// this fails intermittently with EXCEPTION_IN_PAGE_ERROR (0xc0000006),
// crashing gvproxy.exe before it can read its first stdin byte. NTFS
// pages-in works reliably; staging to /mnt/c/... bypasses 9P entirely.
//
// Idempotent: if the staged file already matches the source by sha256, no
// copy happens. The first boot pays one ~13 MB write; later boots are O(hash).
func stageGvproxyExe(srcPath string) (string, error) {
	if err := os.MkdirAll(stagedGvproxyDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", stagedGvproxyDir, err)
	}
	dstPath := filepath.Join(stagedGvproxyDir, filepath.Base(srcPath))

	srcHash, err := sha256OfFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("hash src %s: %w", srcPath, err)
	}
	if dstHash, err := sha256OfFile(dstPath); err == nil && dstHash == srcHash {
		return dstPath, nil
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("copy %s -> %s: %w", srcPath, dstPath, err)
	}
	return dstPath, nil
}

// stageGvproxyConfig writes the embedded gvproxy YAML to the same staged
// directory as the .exe and returns its path in Windows form (e.g.
// `C:\Users\Public\.wsl-vpnfix\gvproxy.yaml`). The Windows form is what
// gvproxy.exe expects when it does os.ReadFile from inside a Windows
// process — argv strings are passed verbatim from Linux interop, no path
// translation, so we must hand it a Windows path up front.
func stageGvproxyConfig() (string, error) {
	if err := os.MkdirAll(stagedGvproxyDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", stagedGvproxyDir, err)
	}
	linuxPath := filepath.Join(stagedGvproxyDir, "gvproxy.yaml")
	if err := os.WriteFile(linuxPath, []byte(gvproxyYAML), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", linuxPath, err)
	}
	return linuxToWindowsPath(linuxPath), nil
}

// linuxToWindowsPath translates a /mnt/<drive>/... DrvFs path into its
// Windows equivalent. Returns the input unchanged if it does not match
// the /mnt/<single-letter>/... shape.
func linuxToWindowsPath(p string) string {
	const prefix = "/mnt/"
	if !strings.HasPrefix(p, prefix) || len(p) < len(prefix)+2 || p[len(prefix)+1] != '/' {
		return p
	}
	drive := strings.ToUpper(p[len(prefix) : len(prefix)+1])
	rest := strings.ReplaceAll(p[len(prefix)+2:], "/", `\`)
	return drive + `:\` + rest
}

func sha256OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".partial"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
