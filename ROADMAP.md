# Roadmap

## v0.2.0 — beginner MVP

- Chinese/English GUI and CLI guided paths.
- Standard-user launch with scoped elevation.
- Portable double-click packages and offline help.
- Key-role onboarding, profile import/export, support redaction, update check.
- Mock/CI safety coverage without changing a personal remote-access host.

## v0.2.1 — first-run and upgrade reliability

- Fresh Windows targets without local public keys can enter the recommendation
  step and switch languages without a stalled render.
- Windows probes and managed actions no longer flash child-process consoles.
- The per-user installer upgrades v0.2.0 in place and keeps one uninstall entry,
  one install directory, and one shortcut set.

## v0.2.2 — UAC result reliability

- Elevated helper paths containing spaces remain a single Windows command-line
  argument.
- UAC cancellation is distinguished from helper launch and execution failure.
- The per-user installer upgrades v0.2.1 in place.

## v0.2.3 — full audit hardening

- Safe phased Tailnet setup and an explicit LAN-only guided route.
- Correct controller-key selection, validation, merge, target-user ownership,
  and idempotent verification.
- SSH configuration precedence, fail-closed authentication/firewall evidence,
  exact CIDR matching, complete rollback, and truthful exit codes.
- Confirmation bound to a canonical Plan digest; integrity-digested,
  ownership-checked, idempotent rollback journals.
- Hash-pinned staged offline execution, hardened bootstraps and offline-pack
  builders, bounded uninstall, report redaction, and patched Go toolchain.
- Shared GUI/CLI elevation protocol, safe privileged output handling, live-PID
  concurrency locks, generated-command parser tests, and HTTPS-only downloads.
- Minimum disposable Windows VM Apply/Verify/Rollback evidence before release.
- Tag-derived release notes and a modularized frontend with transport-aware
  health counting.

## v0.2.4 — personal cards

- Export and import a compact personal card containing controller public keys,
  SSH port, personal labels, network mode, and optional Tailscale bootstrap
  authorization.
- Keep private keys out of the card and redact Tailscale auth keys from plans,
  execution output, and support reports.
- Start imports with a read-only Check and preserve explicit user review and
  UAC confirmation before any system change.
- Upgrade the existing per-user Windows installation in place from v0.2.3.

## v0.3.0 candidates

- Expanded Windows Sandbox/VM interrupted servicing, repeat Apply, cleanup, and
  uninstall-state matrix.
- Disposable Ubuntu and macOS Apply/rollback validation.
- Controller-side real TCP, SSH handshake, authentication, identity, and host
  fingerprint pairing assistant.
- Automatic multi-component offline-pack selection.
- Signed Windows artifacts when certificate infrastructure is available.

## Later

- Signed/notarized macOS desktop distribution.
- Native Linux/macOS desktop installers.
- Managed update channels with explicit rollback.
