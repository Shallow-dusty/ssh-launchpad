# Status

Last locally verified: 2026-07-28

Current release: [`v0.2.0`](https://github.com/Shallow-dusty/ssh-launchpad/releases/tag/v0.2.0)

Release candidate: `v0.2.3`

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
- CLI concurrency uses live PID ownership and token-checked cleanup. Generated
  PowerShell/POSIX commands are parser-tested, and Unix commands are also
  ShellChecked in CI.
- Downloads are HTTPS-only, release notes are resolved from the pushed tag,
  and the frontend is split into focused modules while preserving the wizard.
- Local candidate validation covers Go 1.25.12 vulnerability, race,
  vet/static/security analysis, cross-compilation, ShellCheck, frontend build,
  and 9 browser scenarios. Windows-native Pester, Wails/NSIS, installer upgrade,
  and disposable-VM gates remain pending.

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

### v0.2.3 candidate

The six audit P0 blockers are closed in code and regression tests. Local checks
passed on 2026-07-28, but Windows-native Pester, Wails/NSIS, upgrade/uninstall,
and disposable-VM evidence remain mandatory before tagging. See
[`docs/v0.2.3-security-audit.md`](docs/v0.2.3-security-audit.md) for the closure
matrix, completed checks, and pending gates.

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
  <https://github.com/Shallow-dusty/ssh-launchpad/releases/tag/v0.2.0>
- Tagged commit: `a2ab6a11ac1f854e4a9b0a87ed49278e9e694c70`.
- Release workflow:
  <https://github.com/Shallow-dusty/ssh-launchpad/actions/runs/29679501725>.
- All ten downloaded assets passed the published SHA-256 manifest; the SPDX
  JSON SBOM and archive content checks also passed.
- The downloaded Windows x64 portable CLI reported `0.2.0`, schema `1`, and
  UTF-8 no-BOM JSON. Its Chinese and English double-click launchers both
  completed the read-only wizard smoke with exit code `0`.
- The downloaded unsigned GUI installer completed silent install, first start,
  and uninstall smoke; the installed application was removed afterward.

## Promotion state

`v0.2.0` passed the documented MVP promotion gate. The canonical local checkout
is now `E:\coding\11.SSH-Launchpad`; the GitHub repository and Release URLs
remain the portable runtime contract.

The former `01.Agent-CLI/15.SSH-Launchpad` incubation path is retired. Legacy
`00.scripts/01.SSH快速安装` remains archived, and the 3070 workspace consumes this
release through its device profile without copying the generic implementation.
