# Roadmap

Current focus: finish the **v0.2.6 candidate** validation before expanding the
product. Baseline: [STATUS.md](STATUS.md). Completed work belongs in
[CHANGELOG.md](CHANGELOG.md), not in this backlog.

These are pending items and proposals, not claims of completed acceptance or
an instruction to run system-changing tests on the development workstation.

## P0 — next acceptance round

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

- [ ] **Go 1.26 migration:** align go.mod, local tools and CI; update x/crypto
  to a release fixing the tracked channel-deadlock advisories and rerun
  govulncheck plus regression tests. Review the unmaintained openpgp dependency
  separately; see STATUS for the recorded advisory scope.
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

## Code organization follow-up — separate from release work

The package boundaries are already meaningful; do not rename directories just
for symmetry. If frontend behavior work warrants it, consider extracting the
wizard state/event handling from `frontend/src/main.ts` (currently about 1,000
lines) and splitting views by task, preserving bridge and browser tests. This
is a refactoring candidate, not part of this documentation cleanup or a reason
to rewrite the engine.
