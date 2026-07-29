// scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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
