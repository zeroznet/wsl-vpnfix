<!-- written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7) -->

# Security policy

## Supported versions

Only the latest tagged release on `main` is supported. Older tags are kept for historical reference but receive no security backports.

## Reporting a vulnerability

Please report security issues **privately** via GitHub Security Advisories:

<https://github.com/zeroznet/wsl-vpnfix/security/advisories/new>

If for any reason GitHub Security Advisories are unavailable to you, email `robert@bopko.com` with the subject prefix `[wsl-vpnfix-security]`. PGP is not provided.

Expect an acknowledgement within 7 days. Coordinated disclosure is preferred. If a fix requires coordination with upstream `containers/gvisor-tap-vsock` or with Microsoft (WSL kernel surface), the disclosure timeline will adjust to whichever upstream needs longer.

## What is in scope

The repository's runtime surface, build pipeline, release artifacts, and Windows-side installer (`scripts/install-wslvpnfix.ps1`). See `docs/THREAT-MODEL.md` for adversaries, trust boundaries, and the explicit out-of-scope list.

## What is out of scope

- The WSL kernel itself (Microsoft, shipped via `wsl --update`).
- The `gvisor-tap-vsock` user-space network stack (upstream `containers/gvisor-tap-vsock`).
- The user's corporate VPN client, OS, or network policy.
- Bugs in the user's other WSL distros that traverse the appliance.

Report those upstream.

## Known issues and workarounds

See `docs/SECURITY-AUDIT.md` for the catalogue of internal findings (severity grouped by area), their resolution status, and tracked workarounds.
