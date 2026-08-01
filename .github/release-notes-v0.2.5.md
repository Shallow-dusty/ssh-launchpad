# SSH Launchpad v0.2.5

This release rebuilds the desktop wizard around user tasks and restrains the
visual system, following a deep audit recorded in
`docs/design-audit-2026-08.md`.

## Highlights

- The wizard now has three task steps (check → review → finish; diagnose →
  repair → verify in repair mode) instead of four engine-flavoured steps.
- The plan step drives itself: the detected controller key is preselected, the
  plan builds automatically, and key or network changes rebuild it. The install
  button disables instead of erroring after the fact.
- Network exposure is a single choice that states the consequence of each
  option (Tailscale private network vs LAN only), replacing two unexplained
  buttons.
- Check results list concrete issues instead of an opaque step count, and
  normal "not yet configured" states use neutral styling rather than warning
  colours.
- Plan failures render a retryable error instead of an infinite spinner; a
  failed verify no longer backfills the stale Apply report; check failures
  persist instead of vanishing as toasts.
- Elevated-job polling re-renders only on state or event changes instead of
  rebuilding the DOM twice per second.
- Advanced settings apply live on input; rollback confirmation uses an in-app
  dialog.
- Restrained the visual system: 12px radii, solid surfaces, small layered
  shadows, a single type scale, solid accent buttons, and vendored Lucide
  (ISC) icons.
- Safety simplifications: a rollback journal whose self-computed digest
  mismatches now warns and continues instead of blocking recovery; the desktop
  shell no longer runs a third full Probe+Plan before elevation (the
  authoritative digest check stays inside Apply).

## Distribution

- Windows x64 desktop installer: unsigned.
- Windows x64/ARM64, Linux x64/ARM64, and macOS x64/ARM64 portable CLI bundles.
- macOS artifacts are unsigned and not notarized.
- Verify downloads with the release `checksums.txt`.

No SSH, Tailscale, RDP, or firewall Apply was run on a personal workstation or
production remote host during release validation.