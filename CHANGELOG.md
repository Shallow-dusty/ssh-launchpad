# Changelog

All notable changes are documented here.

## [0.2.3] - 2026-07-25

### Security and reliability

- Gate Tailnet setup into safe phases: install Tailscale first, require an
  online signed-in transport, and only then configure SSH or firewall access.
- Stop silently choosing the first discovered controller key; validate real
  OpenSSH public-key encoding, preserve existing authorized keys, and target
  the correct Windows standard-user or administrators key file.
- Detect and disable conflicting broad Windows inbound SSH rules before adding
  the managed restricted rule; handle all declared Linux CIDRs with exact
  rollback commands.
- Make Windows SSH configuration win before existing directives and `Match`
  blocks, with validation and immediate backup restoration on failure.
- Treat blockers, manual-only actions, and journal persistence failures as
  incomplete work instead of successful Apply or Verify results.
- Harden CLI UAC argument quoting and cancellation classification, scheduled
  task UTF-16 encoding, report redaction, archive extraction, release-version
  paths, offline-pack symlinks, and installer directory deletion boundaries.
- Share one versioned, digest-checked elevation protocol between the desktop
  and CLI, pre-create privileged output files as the standard user, reject
  redirected response paths and reparse points, and preserve file ownership.
- Replace age-only CLI locking with live process ownership and token-checked
  cleanup; validate generated PowerShell and POSIX commands with native parsers
  and ShellCheck.
- Require HTTPS for all initial and redirected downloads, with no runtime
  plaintext override, and select tag-matched release notes at publish time.
- Build with Go 1.25.12, which contains the standard-library security fixes
  identified by the repository vulnerability scan.

### User experience

- Add an explicit LAN-only guided route alongside the recommended Tailscale
  route, accurate phased-plan summaries, LAN connection addresses, and
  blocker explanations.
- Keep cleared key input cleared, prevent navigation away during an active
  install, and list controller keys for explicit selection in the CLI.
- Use one generated-key filename across GUI and CLI, make LAN health counts
  independent of Tailscale, and split the frontend into focused view, browser,
  icon, mock-backend, and model modules without changing the guided flow.

## [0.2.2] - 2026-07-25

### Fixed

- Preserve quoting for elevated-helper request, response, and event paths when
  their per-user cache directory contains spaces.
- Treat only the explicit Windows `ERROR_CANCELLED` result as a cancelled UAC
  prompt. A helper launch or execution failure now remains a failed install and
  keeps its diagnostic instead of incorrectly blaming the user.

## [0.2.1] - 2026-07-25

### Fixed

- Keep empty profile collections present across the Go/Wails JSON boundary so a
  fresh computer with no local `.pub` files can continue to the recommendation
  step.
- Normalize profiles before rendering and after imports, preventing language
  changes from re-triggering a failed wizard render.
- Hide Windows child-process consoles during checks, Apply, Verify, and
  rollback while preserving captured command output and explicit UAC prompts.
- Reuse the existing per-user install directory, uninstall registration, and
  shortcuts during upgrades; running copies are blocked with a close-and-retry
  prompt instead of creating a parallel installation.

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
