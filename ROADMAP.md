# Roadmap

Code consolidation, security upgrade and local regression for the **v0.2.6
candidate** are complete. Remaining work is listed below; cloud-host acceptance
is deliberately deferred. Baseline:
[STATUS.md](STATUS.md). Completed release work belongs in
[CHANGELOG.md](CHANGELOG.md), not in this backlog.

These are pending items and proposals, not claims of completed acceptance or
an instruction to run system-changing tests on the development workstation.

Completed consolidation/security work and local results are recorded in
[CHANGELOG](CHANGELOG.md) and
[the October evidence record](docs/records/code-consolidation-2026-10-09.md),
not retained as completed checkboxes in this pending-only backlog.

## Deferred — next real-host acceptance round

Refresh the current candidate's local checks and native CI first, then use
disposable targets with an independent recovery path. The recorded procedure
and first-round findings are in
[the acceptance report](docs/records/acceptance-2026-09.md#next-acceptance-round).

- [ ] **Unelevated Windows Apply gate:** run from a real standard-user session
  (or a deliberately filtered Administrator token); expect exit 4 and no
  configuration, firewall, or service changes.
- [ ] **Windows 10/11 client flow:** exercise official OpenSSH + Tailscale
  setup with winget present, actual GUI/UAC confirmation and cancellation,
  and a host without WebView2.
- [ ] **Installer recheck:** verify missing-marker refusal returns exit 68;
  record silent-uninstall leftovers and upgrade/uninstall behavior.
- [ ] **Unix live acceptance:** Linux systemd + UFW, then firewalld, and macOS;
  cover Apply / Verify / repeated Apply / Rollback, with explicit UFW raw-rule
  limitations rather than an assumed complete inventory.

The full release procedure is maintained only in
[the release checklist](docs/release-verification.md). Record test dates,
commits, evidence, and any consciously deferred platform coverage before
making a release decision.

## P1 — maintenance and known gaps

- [ ] **Winget-less mixed dependency strategy:** design per-component download
  selection so OpenSSH capability and a pinned offline Tailscale installer do
  not require manually switching one global strategy.
- [ ] **Interruption acceptance (optional stress round):** kill/power-loss
  during Apply on disposable hosts; validate write-ahead journals and recovery
  against real interruptions, beyond existing unit fixtures.

## Candidate future features — not committed to a version

- Controller-side connection assistant: real TCP/SSH handshake,
  authentication, identity, and host-fingerprint pairing.
- Automatic multi-component offline-pack selection.
- Signed Windows artifacts when certificate infrastructure is available.
- Signed/notarized macOS desktop distribution and native Linux/macOS
  desktop installers.
- Managed update channels with explicit rollback.

Do not rename directories solely for symmetry or turn the code cleanup into
an engine rewrite. Real-host acceptance is deliberately deferred for this round;
its open boundaries remain recorded above, not marked as passed.
