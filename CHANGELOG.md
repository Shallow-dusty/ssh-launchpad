# Changelog

All notable changes are documented here.

## [0.2.0] - 2026-07-19

### Added

- Beginner-first Chinese/English desktop wizard with persisted language and
  system-language detection.
- Four-step Check, Recommend, Safe Install, and Test flow with plain-language
  green/yellow/red outcomes and expandable technical details.
- Standard-user Windows launch with a request-integrity-checked UAC helper and
  progress returned to the main window.
- Public-key discovery, safe generation, import/export, controller/target role
  guidance, pairing file, and explicit host-fingerprint verification advice.
- Beginner CLI wizard, bilingual help/errors, stable JSON mode, UTF-8 console
  handling, non-interactive detection, and process lock.
- Bilingual Windows double-click launchers, macOS `.command`, Linux `.desktop`,
  offline help, and per-portable bundle checksums.
- YAML/JSON profile import/export, redacted support report, stable-channel update
  check, and local offline dependency pack commands.
- E2E coverage for first run, language persistence, profile import, UAC cancel,
  idempotent revisit, narrow layout, keyboard navigation, and package contents.

### Security and distribution

- Tailnet-only remains the recommended exposure and self-cut is blocked by
  default.
- Machine reports remain stable English JSON; shareable GUI exports are
  redacted.
- Windows desktop artifacts remain unsigned. macOS artifacts are not notarized.
- No real Apply was run on a personal workstation or remote host during release
  validation.

## [0.1.0] - 2026-07-18

### Added

- Cross-platform Check, Plan, Apply, Verify, and Rollback engine.
- JSON report and stable exit-code contract.
- Windows, Linux, macOS, and separate WSL planning.
- Optional Tailscale transport with tailnet-only default exposure.
- Self-cut detection, delayed risky actions, journals, and rollback.
- Verified resumable download core and five download strategies.
- Standalone PowerShell 5.1 and POSIX shell bootstraps.
- Accessible Wails desktop Studio.
- Multi-platform CI, browser tests, package smoke tests, checksums, and SBOM.

### Distribution notes

The first release is not code-signed or notarized. No Apply was run against a
production or personal remote-access host as part of release validation.
