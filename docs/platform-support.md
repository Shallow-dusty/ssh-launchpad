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

## Current native boundary

- Real Apply was not run on the development workstation or a personal remote
  host.
- Windows UAC request integrity, cancellation, progress return, and mock Apply
  are tested; the final disposable-VM servicing/interrupt matrix remains v0.3.
- Linux/macOS adapters are exercised by native CI and generated-command tests,
  not by changing a production host.
- Windows artifacts are unsigned. macOS artifacts are not signed/notarized and
  may require the user to approve the downloaded file in system settings.
- Linux desktop entry launch depends on the file manager honoring `Terminal=true`
  and the executable bit; the direct CLI remains the portable fallback.
