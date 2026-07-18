SSH Launchpad v0.1.0 is the first installable release of the cross-platform SSH
bootstrap and recovery studio.

Highlights:

- read-only Check and Plan, explicitly confirmed Apply, non-elevating Verify;
- shared engine for CLI and the Wails desktop Studio;
- separate Windows, Linux, macOS, and WSL target models;
- optional tailnet-only, LAN, or custom-CIDR exposure;
- self-cut protection, delayed risky actions, journals, and rollback;
- standalone PowerShell 5.1 and POSIX shell bootstraps;
- versioned JSON reports and stable exit codes.

Distribution:

- CLI archives are provided for Windows, Linux, and macOS on amd64 and arm64.
- The bootstrap bundle installs from GitHub Releases, an explicit HTTPS mirror
  or proxy, an offline bundle, or verified cache.
- The Windows desktop installer embeds the WebView2 bootstrapper.
- `checksums.txt` and an SPDX JSON SBOM are included.

The v0.1.0 desktop installer is not code-signed. macOS artifacts are not
notarized. Verify the SHA-256 manifest before installation. No real device
profile, private key, token, or diagnostic log is included.
