# Status

Last verified: 2026-07-19

Current release: [`v0.2.0`](https://github.com/Shallow-dusty/ssh-launchpad/releases/tag/v0.2.0)

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

## Validation

The tagged source is covered by Go unit/vet checks, Pester and shell checks,
six browser wizard E2E scenarios, a Wails/NSIS build, archive/package smoke,
real extracted Windows CLI and bilingual launcher smoke, plus silent
install/first-start/uninstall smoke. See
[`docs/v0.2-acceptance.md`](docs/v0.2-acceptance.md).

No SSH, Tailscale, RDP, or firewall Apply was run against the development
workstation or any remote host. Linux and macOS system-changing behavior is
validated through generated-command tests and native CI runners rather than a
real target. The Windows installer is not code-signed, and macOS artifacts are
not notarized.

## Release assets

- Unsigned Windows x64 GUI installer.
- Windows x64/ARM64, Linux x64/ARM64, and macOS x64/ARM64 portable CLI bundles.
- Standalone bilingual bootstrap bundle.
- SHA-256 manifest and SPDX JSON SBOM.

## Promotion state

`v0.2.0` is the MVP promotion gate for the future standalone project location.
The current checkout path is development evidence only and is not a runtime
contract. The separate workspace migration, legacy-entry compatibility, and
device-profile consumption remain root-workspace responsibilities after the
Release is verified.
