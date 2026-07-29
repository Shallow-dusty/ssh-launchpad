# SSH Launchpad v0.2.4

This release adds portable personal cards and optional unattended Tailscale
bootstrap for computers you control.

## Highlights

- Create a `.sshlaunchpad-card` containing controller SSH public keys, the SSH
  port, personal labels, Tailnet/LAN mode, and an optional Tailscale auth key.
- Import a card from the home page and begin with the existing read-only
  computer check before reviewing any planned system changes.
- Use a profile or card-supplied Tailscale auth key during confirmed Apply so a
  fresh computer can join the tailnet before SSH and its restricted firewall
  rule are configured.
- Reject private-key material, malformed public keys, invalid auth keys,
  oversized cards, and trailing JSON values while ignoring unknown fields for
  forward compatibility.
- Keep auth keys out of reviewable plans, journals, exported YAML profiles,
  action results, and support reports. Elevation consumes its restricted
  request before Apply; use one-time or short-lived keys because interrupted
  workflows can leave crash residue and `tailscale up` briefly exposes the key
  in child-process arguments.
- Upgrade the existing per-user v0.2.3 Windows installation in place without a
  second app entry or install directory.

## Distribution

- Windows x64 desktop installer: unsigned.
- Windows x64/ARM64, Linux x64/ARM64, and macOS x64/ARM64 portable CLI bundles.
- macOS artifacts are unsigned and not notarized.
- Verify downloads with the release `checksums.txt`.

No SSH, Tailscale, RDP, or firewall Apply was run on a personal workstation or
production remote host during release validation.
