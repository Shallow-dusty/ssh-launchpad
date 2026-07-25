# SSH Launchpad v0.2.4

This release adds portable personal cards for quickly onboarding computers you
control.

## Highlights

- Create a `.sshlaunchpad-card` containing controller SSH public keys, the SSH
  port, personal labels, Tailnet/LAN mode, and an optional Tailscale auth key.
- Import a card from the home page and begin with the existing read-only
  computer check before reviewing any planned system changes.
- Use a card-supplied Tailscale auth key during the confirmed Apply flow so a
  fresh Windows computer can install or authorize Tailscale before SSH and its
  restricted firewall rule are configured.
- Reject private keys, malformed public keys, unknown fields, invalid auth
  keys, oversized cards, and trailing JSON values.
- Mask Tailscale auth keys in the UI, keep them out of inspectable plans and
  journals, and redact them from execution output and support reports.
- Upgrade the existing per-user v0.2.3 Windows installation in place without a
  second app entry or install directory.

## Distribution

- Windows x64 desktop installer: unsigned.
- Windows x64/ARM64, Linux x64/ARM64, and macOS x64/ARM64 portable CLI bundles.
- macOS artifacts are unsigned and not notarized.
- Verify downloads with the release `checksums.txt`.

No SSH, Tailscale, or firewall Apply was run on a personal workstation or
production remote host during release validation.
