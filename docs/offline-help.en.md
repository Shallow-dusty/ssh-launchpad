# SSH Launchpad offline help

## Simplest path

Windows desktop users should choose the installer, open it, and follow the guided UI. For servers or computers without a desktop, extract the complete portable ZIP and open `Start SSH Launchpad.cmd`.

## Do not mix up the keys

- You are setting up the target computer.
- It needs the controller computer's `.pub` public key.
- The private key has no `.pub` suffix and must stay on the controller. Never copy, paste, upload, or store it in a profile.
- On the first connection, compare the host fingerprint instead of silently accepting an unknown fingerprint.

## No network

The tool runs offline. Bootstrap command options can select an explicit proxy or HTTPS mirror, or use an offline release asset next to its `checksums.txt`. Profile download fields are not implicitly forwarded to dependency package managers.

Only the Windows Tailscale adapter currently accepts a SHA-256-pinned `offlineBundle`. Install offline OpenSSH separately through trusted platform servicing, then Check/Plan again. Unsupported combinations are blocked. Never disable TLS or security software to bypass verification.

## Permission, cancellation, and recovery

Start the Windows GUI as a normal user; system changes/recovery request UAC when needed. Follow the Unix CLI's administrator guidance separately. Cancelling permission stops further work.

After partial failure, later steps stop and enabled auto-rollback attempts reversible recovery. The failed-Apply page and advanced settings offer report export and Recover reversible changes. Installed packages/logins may remain. Inspect recovery results before retrying; keep the window open while an operation is still active.

## Remote-session safety

If the only connection depends on SSH or Tailscale, SSH Launchpad blocks actions that could disconnect itself. Run locally on the target, prepare a second channel, or complete external verification from another computer.

`--schedule-risky` is an in-process delay retaining the mutation lock, not a detached task; keep the process running. Legacy scheduled journals require manual task verification. Custom UFW raw rules are not completely inventoried automatically.

A successful finish means local checks passed. Still test public-key authentication from the controller and compare the host fingerprint.

This version is not code-signed. When the operating system warns, verify the Release SHA-256 first; disabling SmartScreen or security software is not recommended.
