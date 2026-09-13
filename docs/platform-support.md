# Platform support

| Target | Portable CLI | Beginner launcher | Apply model | Desktop | v0.2.0 evidence |
| --- | --- | --- | --- | --- | --- |
| Windows 10/11 x64 | Yes | 中文/English `.cmd`; direct EXE wizard | OpenSSH Windows Capability, service, scoped firewall, on-demand UAC | Unsigned x64 installer | local build/read-only checks, mocks, Go/Pester/E2E, Windows CI |
| Windows 10/11 ARM64 | Yes | 中文/English `.cmd`; direct EXE wizard | Same generated adapter | No installer | cross-build and unit/CI package smoke |
| WSL | Yes, distinct target | terminal CLI | Linux distribution service; Windows state not conflated | No | planner/adapter tests and generated commands |
| Linux x64/ARM64 | Yes | terminal `.desktop` plus CLI wizard | common systemd distributions; sudo on demand | No native GUI package | native CI, ShellCheck, unit and package tests |
| macOS Intel/Apple Silicon | Yes | `.command` plus CLI wizard | system OpenSSH and launchd; sudo on demand | No native GUI package | native CI, unit and package tests |

PowerShell bootstrap supports Windows PowerShell 5.1 and PowerShell 7. It sets
UTF-8 only for its current process so machine-wide console configuration is not
changed. The Go CLI uses Windows Console APIs and restores prior code pages on
exit. JSON files are UTF-8 without BOM.

On Linux/macOS, `LANG`/`LC_ALL` select Chinese only for a UTF-8 Chinese locale.
A non-UTF-8 locale falls back to English/ASCII. Non-TTY/CI execution never waits
for prompts or emits animation/color; `NO_COLOR` is honored because the CLI
does not require ANSI color.

The table's evidence column records the original v0.2.0 baseline, not a new
execution of every CI/native test. Current candidate evidence is in
[the September audit](audit-2026-09.md).

## Current native boundary

- Real Apply was not run on the development workstation or a personal remote
  host.
- Windows UAC request integrity, cancellation, progress return, and mock Apply
  are tested with mocks. Disposable-VM Apply/Verify/Rollback and installer
  upgrade smoke remain outstanding candidate acceptance gates; no version label
  by itself proves these passed.
- Linux/macOS adapters are exercised by native CI and generated-command tests,
  not by changing a production host. Native CI is configured; its latest remote
  run was not checked as part of local verification.
- UFW/firewalld evidence is deliberately narrow: UFW requires a readable
  verbose inventory with a deny/reject incoming default and only fully
  understood inbound rules; firewalld requires one active default zone, no
  service/policy/direct/source-port/forwarding entries, no runtime/persistent
  drift, and only exact source/port `accept` rich rules. Anything else is a
  plan blocker rather than an inferred safe state **when visible in those
  supported inventories**. UFW before/after raw rules and independent
  nftables/iptables rules are not read by the current adapter; such customized
  hosts require external inventory and are not fully validated targets.
- SSH authentication is probed with the global `sshd -T` dump plus a
  conservative source-policy inspection. Unsupported `Match` blocks, custom
  `ListenAddress`, a `Port` inside an included file, recursive includes, or
  other connection-dependent policy are treated as unchecked and fail closed
  as plan blockers. The stock Win32-OpenSSH `Match Group administrators` /
  `administrators_authorized_keys` redirection is the only modeled exception.
  Extra effective SSH ports also force configuration review instead of being
  mistaken for convergence on the requested port.
- Windows artifacts are unsigned. macOS artifacts are not signed/notarized and
  may require the user to approve the downloaded file in system settings.
- Linux desktop entry launch depends on the file manager honoring `Terminal=true`
  and the executable bit; the direct CLI remains the portable fallback.
