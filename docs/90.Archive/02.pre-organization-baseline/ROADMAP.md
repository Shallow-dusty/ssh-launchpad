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
- Tag-derived release notes and a modularized frontend with transport-aware
  health counting.

## v0.2.4 — personal cards and unattended bootstrap

- Export and import a compact personal card (controller public keys, SSH
  port, display labels, network mode, and an optional Tailscale auth key) so
  a new device can be onboarded without editing YAML.
- `transport.authKey` enables one-pass unattended Tailnet bootstrap; the key
  is kept out of plans, journals, exported profiles, and reports, and the
  trade-offs are documented in the threat model.
- Without a card or auth key, the phased sign-in flow is unchanged.

## v0.2.5 — deep audit and task-based wizard

- Rebuilt the desktop wizard around user tasks (check → review → finish;
  repair mode: diagnose → repair → verify) instead of engine stages, with a
  self-driving plan step, preselected controller keys, consequence-labelled
  network exposure, concrete check-issue lists, and persistent error states.
- Restrained the visual system (12px radii, solid surfaces, a single type
  scale, solid accent buttons) and vendored Lucide icons.
- Applied advanced settings live; rollback confirmation moved to an in-app
  dialog.
- Downgraded the rollback-journal digest mismatch to a warning and removed
  the GUI's third pre-elevation Probe+Plan (authoritative digest check stays
  inside Apply), following `docs/design-audit-2026-08.md`.

## v0.3.0 candidates

- Real-target validation in disposable VMs (Apply/Verify/Rollback, repeat
  Apply, upgrade, uninstall) on Windows, Ubuntu, and macOS.
- Controller-side real TCP, SSH handshake, authentication, identity, and host
  fingerprint pairing assistant.
- Automatic multi-component offline-pack selection.
- Signed Windows artifacts when certificate infrastructure is available.

## Later

- Signed/notarized macOS desktop distribution.
- Native Linux/macOS desktop installers.
- Managed update channels with explicit rollback.
