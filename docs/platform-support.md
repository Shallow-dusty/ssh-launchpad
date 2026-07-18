# Platform support

| Target | CLI and Check/Plan | Apply adapter | Service model | Desktop | v0.1.0 evidence |
| --- | --- | --- | --- | --- | --- |
| Windows 10/11 amd64 | Supported | Supported | OpenSSH capability, `sshd`, Windows Firewall | Installer | local build/tests + CI |
| Windows 11 arm64 | Supported | Supported | same | portable build where runner permits | cross-build + CI |
| Linux amd64/arm64 | Supported | systemd distributions | `sshd`/`ssh`, ufw/firewalld | source build only in v0.1.0 | unit + native CLI CI |
| macOS amd64/arm64 | Supported | supported commands | system OpenSSH + launchd | source build only in v0.1.0 | unit + native CLI CI |
| WSL 1/2 | Explicit target | systemd where enabled | Linux instance only | use host UI/CLI | planner/unit tests |

PowerShell bootstrap syntax supports Windows PowerShell 5.1 and PowerShell 7.
The POSIX bootstrap targets `/bin/sh` and relies on `curl` or `wget`, a
SHA-256 tool, and `tar`/`unzip`.

Apply support does not mean every distribution package name or local security
product is known. Unsupported package managers or firewall providers fail
closed with an explicit action/report instead of silently changing policy.

The desktop layer uses the operating system WebView. Windows requires WebView2.
Future Linux packages will require the WebKitGTK dependency expected by Wails;
future macOS packages will use the native WebKit runtime. These two desktop
packages are not release artifacts in `v0.1.0`.
