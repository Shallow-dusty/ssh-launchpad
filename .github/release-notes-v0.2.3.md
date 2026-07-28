# SSH Launchpad v0.2.3

This release hardens the guided SSH setup flow after a full cross-layer audit.

## Highlights

- Phases Tailnet setup so a missing transport is installed and signed in
  before SSH or firewall exposure is changed.
- Adds an explicit LAN-only guided route alongside the recommended Tailscale
  route.
- Requires an explicit controller-key choice, parses real OpenSSH public keys,
  preserves existing access, and targets the correct account key file.
- Restores SSH configuration and service state after validation or restart
  failure, and remediates conflicting broad Windows firewall rules before
  creating the managed restricted rule.
- Distinguishes a real UAC cancellation from helper failures and preserves
  arguments whose paths contain spaces.
- Binds confirmation to a canonical plan digest and carries it through one
  digest-checked GUI/CLI privilege protocol, with fixed ordinary-user-owned
  response files and reparse-point-safe elevated writes.
- Replaces age-only process locking, validates generated PowerShell/POSIX
  commands with native parsers and ShellCheck, and resolves release notes from
  the pushed tag.
- Fails closed on unknown authentication/firewall evidence and extra firewall
  scopes; invalid journals return nonzero and rollback is idempotent.
- Hardens downloads, hash-pinned offline execution, bootstraps, offline packs,
  archive paths, report redaction, installer upgrades, and bounded uninstall.

## Distribution

- Windows x64 desktop installer: unsigned.
- Windows x64/ARM64, Linux x64/ARM64, and macOS x64/ARM64 portable CLI bundles.
- macOS artifacts are unsigned and not notarized.
- Verify downloads with the release `checksums.txt`.

No SSH, Tailscale, or firewall Apply was run on a personal workstation or
production remote host during release validation.
