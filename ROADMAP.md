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
- SSH configuration precedence, broad-firewall conflict remediation, complete
  CIDR rollback, journal integrity, and truthful blocker reporting.
- Hardened bootstraps, offline-pack builders, cross-platform launchers,
  installer path boundaries, report redaction, and patched Go toolchain.
- Shared GUI/CLI elevation protocol, safe privileged output handling, live-PID
  concurrency locks, generated-command parser tests, and HTTPS-only downloads.
- Tag-derived release notes and a modularized frontend with transport-aware
  health counting.

## v0.3.0 candidates

- Disposable Windows Sandbox/VM real Apply, interrupted servicing, repeat Apply,
  cleanup, and uninstall-state matrix.
- Disposable Ubuntu and macOS Apply/rollback validation.
- Controller-side real TCP, SSH handshake, authentication, identity, and host
  fingerprint pairing assistant.
- Automatic multi-component offline-pack selection.
- Signed Windows artifacts when certificate infrastructure is available.

## Later

- Signed/notarized macOS desktop distribution.
- Native Linux/macOS desktop installers.
- Managed update channels with explicit rollback.
