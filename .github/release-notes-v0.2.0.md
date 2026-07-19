# SSH Launchpad v0.2.0

This release turns SSH Launchpad into a beginner-facing remote connection
assistant while keeping the expert CLI and stable JSON automation contract.

## Recommended for Windows

Download **`SSH-Launchpad_0.2.0_Windows_x64_Installer_UNSIGNED.exe`**. Install,
open, and choose **“让这台电脑可以被远程连接 / Let me connect to this computer
remotely.”**

The installer is currently **unsigned**. Download only from this Release and
verify `checksums.txt`. Disabling SmartScreen or security software is not a
normal workaround.

## Portable / server / repair

Choose the matching **`*_Portable`** archive. Windows users can open
`开始使用 SSH Launchpad.cmd` or `Start SSH Launchpad.cmd` after extracting the
complete ZIP. macOS includes a `.command` launcher and Linux includes a
terminal `.desktop` launcher.

Every portable bundle contains:

- a standalone CLI binary with no Go/Node/Wails runtime requirement;
- Chinese and English offline help;
- an example profile;
- a per-bundle SHA-256 manifest;
- offline dependency-pack tools and format documentation.

SSH Launchpad itself runs offline. A fully offline Apply on a machine missing
OpenSSH or optional Tailscale also needs user-supplied, licensed platform
payloads.

## Highlights

- System-language selection with persistent 中文 / English switch.
- Four-step beginner GUI and no-argument CLI wizard.
- Clear target/computer roles; private keys are never copied or profiled.
- Standard-user Windows launch with on-demand UAC and progress return.
- Plain-language plan, failure, retry, no-change, recovery, and external test
  guidance.
- Self-cut protection, idempotent reruns, single-instance protection, redacted
  support exports, and stable-channel update checking.

## Verification boundary

Check, Plan, mock Apply, rollback behavior, UAC boundary, GUI interaction,
packaging, checksum, and multi-platform command generation are tested. This
release did **not** perform a real SSH/Tailscale/firewall Apply on a personal or
production host. Windows desktop is unsigned; macOS is not notarized.
