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
  frontend typecheck/build, and 13 browser scenarios. The native Windows
  gates (Pester, Wails/NSIS, upgrade smoke, disposable-VM matrix) remain open
  before any v0.2.4 tag.

## v0.2.3 audit hardening

- Tailnet setup is phased: install transport first, require online sign-in, then
  configure SSH and a restricted firewall rule. The guided GUI also offers an
  explicit LAN-only route.
- Controller keys require explicit selection, are parsed as real OpenSSH keys,
  merge without deleting existing access, and target the correct Windows
  standard-user or administrators file.
- Windows SSH configuration is inserted before existing directives and
  `Match` blocks; broad inbound rules are detected and disabled before the
  managed scoped rule is created.
- Linux/WSL key ownership follows the invoking user through `sudo`; every
  declared firewall CIDR has a matching rollback operation.
- Plan blockers, unknown authentication/firewall evidence, extra exposure
  scopes, UAC helper failures, and journal read/setup/persistence errors can no
  longer be reported as successful completion.
- Bootstraps, release paths, archive extraction, hash-pinned offline execution,
  offline-pack symlinks, report redaction, macOS launcher line endings, and
  bounded non-recursive uninstall are hardened.
- Desktop and CLI confirmation is bound to a canonical plan digest carried by
  one request/response protocol, with fixed per-request output locations,
  ordinary-user-owned response files, and reparse-point-safe elevated writes.
- CLI concurrency uses an exclusive-create PID lock with stale-PID recovery.
  Generated
  PowerShell/POSIX commands are parser-tested, and Unix commands are also
  ShellChecked in CI.
- Downloads are HTTPS-only, release notes are resolved from the pushed tag,
  and the frontend is split into focused modules while preserving the wizard.
- Release validation covers Go 1.25.12 vulnerability, race,
  vet/static/security analysis, cross-compilation, ShellCheck, Windows
  PowerShell 5.1 and PowerShell 7 Pester, frontend build, 9 browser scenarios,
  Wails/NSIS, a verified v0.2.0-to-v0.2.3 installer upgrade, package/SBOM
  checksums, and history plus extracted-archive secret scans.

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

### v0.2.3 release

The six audit P0 blockers are closed in code and regression tests. The native
release gates passed on 2026-07-29, including Windows PowerShell 5.1 and
PowerShell 7 Pester, Wails/NSIS, a checksum-verified v0.2.0-to-v0.2.3
upgrade/uninstall smoke, portable CLI execution, package checksums, SBOM, and
secret/identity scans. See
[`docs/v0.2.3-security-audit.md`](docs/v0.2.3-security-audit.md) for the closure
matrix, completed checks, and explicit validation boundary.

### Published v0.2.0 baseline

The published source passed Go unit/vet checks, Pester and shell checks, browser
wizard E2E scenarios, a Wails/NSIS build, archive/package smoke, extracted
Windows CLI and bilingual launcher smoke, plus silent
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

## Release evidence

- Published Release:
  <https://github.com/Shallow-dusty/ssh-launchpad/releases/tag/v0.2.3>
- The immutable `v0.2.3` tag resolves to
  `42248be58e693da5f58c26b7a2fc7eada71a5677`.
- All nine locally staged assets passed the SHA-256 manifest; all six portable
  bundle manifests passed 74 inner-file checks, including UTF-8 Chinese
  filenames. The SPDX JSON SBOM and archive content checks also passed.
- The original tag workflow built the eight runtime artifacts successfully but
  its publish job stopped on two smoke-check assumptions about CI archives
  having a top-level directory. Those validators were fixed in
  [PR #1](https://github.com/Shallow-dusty/ssh-launchpad/pull/1) and
  [PR #2](https://github.com/Shallow-dusty/ssh-launchpad/pull/2), with green
  Windows, Linux, macOS, UI, vulnerability, and secret-scan CI.
- Publication reused the eight artifacts downloaded from the original Actions
  run, added the verified SPDX SBOM and regenerated `checksums.txt`, then
  re-downloaded all ten Draft Release files. Every uploaded SHA-256 matched the
  reviewed staging set before the Release was promoted to Latest.
- The extracted Windows x64 portable CLI reported `0.2.3`, schema `1`, and
  UTF-8 no-BOM JSON during a read-only Check.
- The checksum-verified v0.2.0 installer upgraded in place to the unsigned
  v0.2.3 installer, preserved an unrelated sentinel, updated version resources,
  retained one shortcut/uninstall set, and cleanly uninstalled afterward.

## Promotion state

`v0.2.3` passed the documented audit-hardening release gate. The canonical
local checkout is now `E:\coding\11.SSH-Launchpad`; the GitHub repository and
Release URLs remain the portable runtime contract.

The former `01.Agent-CLI/15.SSH-Launchpad` incubation path is retired. Legacy
`00.scripts/01.SSH快速安装` remains archived, and the 3070 workspace consumes this
release through its device profile without copying the generic implementation.
