# SSH Launchpad offline help

## Simplest path

Windows desktop users should choose the installer, open it, and follow the guided UI. For servers or computers without a desktop, extract the complete portable ZIP and open `Start SSH Launchpad.cmd`.

## Do not mix up the keys

- You are setting up the target computer.
- It needs the controller computer's `.pub` public key.
- The private key has no `.pub` suffix and must stay on the controller. Never copy, paste, upload, or store it in a profile.
- On the first connection, compare the host fingerprint instead of silently accepting an unknown fingerprint.

## No network

Retry later, configure an explicit proxy or HTTPS mirror, or use an offline asset next to its `checksums.txt`. A download that fails verification is never executed. Do not disable TLS or security software to bypass an error.

## Permission, cancellation, and recovery

Start as a normal user. UAC/sudo is requested only for installation. Cancelling permission stops further work. After partial failure, later steps stop and reversible work is restored from the execution record; advanced mode can export the report or restore the last run.

## Remote-session safety

If the only connection depends on SSH or Tailscale, SSH Launchpad blocks actions that could disconnect itself. Run locally on the target, prepare a second channel, or complete external verification from another computer.

v0.2.0 is not code-signed. When the operating system warns, verify the Release SHA-256 first; disabling SmartScreen or security software is not recommended.
