# Status

Last locally verified: 2026-07-29

Current release: [`v0.2.3`](https://github.com/Shallow-dusty/ssh-launchpad/releases/tag/v0.2.3)

## Unreleased toward v0.2.4

- Personal cards (`.sshlaunchpad-card`) carry controller public keys, SSH
  port, display labels, network mode, and an optional Tailscale auth key from
  the controller to a new device, prefilling the wizard and starting from the
  read-only Check. Unknown card fields are ignored for forward compatibility.
- `transport.authKey` enables one-pass unattended Tailnet bootstrap; the key
  is materialized only inside Apply and redacted from plans, journals,
  reports, and exported profiles (see `docs/threat-model.md`).
- Validation so far: Go unit/race/vet, Windows/macOS cross-compilation,
  frontend typecheck/build, and 13 browser scenarios. Windows-native checks
  (Pester, Wails/NSIS, upgrade smoke) remain open before any v0.2.4 tag.

## Current product

- A beginner-first Chinese/English desktop wizard for setting up, checking, and
  repairing remote access without editing YAML or starting as administrator.
- A matching beginner CLI wizard, stable non-interactive JSON mode, and
  bilingual double-click launchers.
- A shared Go Check/Plan/Apply/Verify/Rollback engine for Windows, Linux, macOS,
  and a distinct WSL target layer.
- Public-key onboarding that distinguishes the target computer from the
  controller, never transports private keys, and keeps host-fingerprint
  verification visible.
- Tailnet-only recommended exposure, source- and port-aware firewall planning,
  self-cut protection, process locks, rollback journals, and external
  verification guidance.
- Standalone portable bundles, bootstraps, offline help, and dependency-pack
  builders. The tool itself runs offline; installing a missing OpenSSH or
  Tailscale package fully offline requires a user-supplied, checksummed payload.

## Validation boundary

- No SSH, Tailscale, RDP, or firewall Apply was run against the development
  workstation or any remote host. Linux and macOS system-changing behavior is
  validated through generated-command tests and native CI runners rather than
  a real target.
- The Windows installer is not code-signed, and macOS artifacts are not
  notarized.
- What was verified for each release is recorded in `CHANGELOG.md`.

## Release assets

- Unsigned Windows x64 GUI installer.
- Windows x64/ARM64, Linux x64/ARM64, and macOS x64/ARM64 portable CLI bundles.
- Standalone bilingual bootstrap bundle.
- SHA-256 manifest and SPDX JSON SBOM.
