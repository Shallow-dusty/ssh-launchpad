# Status

Last verified: 2026-07-18

Current release: [`v0.1.0`](https://github.com/Shallow-dusty/ssh-launchpad/releases/tag/v0.1.0)

## Implemented

- Shared Go Check/Plan/Apply/Verify/Rollback engine and JSON report schema.
- Windows, Linux, macOS, and distinct WSL platform adapters.
- Port- and source-scope-aware firewall planning.
- Optional Tailscale transport and tailnet/LAN/custom exposure profiles.
- Self-cut detection, delayed risky actions, rollback journals, and external
  verification contract.
- Official, mirror, proxy, offline, and cache download strategies with HTTPS,
  retry/backoff, resume support in the core downloader, and SHA-256 validation.
- PowerShell 5.1-compatible and POSIX shell standalone bootstraps.
- Wails Studio with responsive, accessible status, plan, progress, verify,
  recovery, and advanced views.
- Unit, Pester, ShellCheck, browser E2E, package smoke, and multi-OS CI coverage.

## Current validation boundary

The local Windows build and dry/read-only stages are tested. No Apply was run
against this workstation or any remote host. Linux and macOS behavior is
validated by unit tests, generated commands, and native CI runners rather than
by changing a real target. Release artifacts are not code-signed or notarized
in `v0.1.0`.

## Release evidence

- Release workflow: <https://github.com/Shallow-dusty/ssh-launchpad/actions/runs/29652426746>
- Assets: six CLI archives, standalone bootstrap bundle, Windows amd64 desktop
  installer, SHA-256 manifest, and SPDX JSON SBOM.
- Published tag commit: `31d564339785c8429f159f8233ae04f5c7fe418c`.

## Next action

Begin the `v0.2.0` disposable-target matrix without changing the `v0.1.0` tag.
