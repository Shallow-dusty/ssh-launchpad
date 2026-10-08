# Development

Requirements: Go 1.25+, Node 22+, pnpm 10+, Wails 2.13, and NSIS for a
Windows installer.

```text
go test -p 1 ./...
go test -p 1 -race ./...
go vet ./...
# On Unix, generated command tests also run sh -n and ShellCheck.
cd frontend
pnpm install --frozen-lockfile
pnpm run build
pnpm run test:e2e
cd ..
wails build
```

Build the versioned per-user Windows upgrade installer through the project
wrapper so the custom NSIS identity/upgrade checks and GUI version resources
are applied:

```text
pwsh -NoProfile -File scripts/build-windows-installer.ps1 -Version 0.2.6
```

For Windows Pester 5 runs, keep the XML report out of the repository root:

```powershell
New-Item -ItemType Directory -Force -Path build/audit/pester | Out-Null
$config = New-PesterConfiguration
$config.Run.Path = 'tests'
$config.Run.Exit = $true
$config.TestResult.Enabled = $true
$config.TestResult.OutputPath = 'build/audit/pester/testResults.xml'
Invoke-Pester -Configuration $config
```

Serialize Go test packages with `-p 1`: separate test binaries exercise the
same system-wide mutation lock. Package-local race detection and intentional
lock-contention tests remain enabled. Do not run another mutation test suite
or a real Apply concurrently.

Never run tests that change SSH, Tailscale, RDP, or firewall state on a real
host; cover those paths with mocks, command parsers, and generated-command
tests. When touching recovery, also exercise failure before, during, and after
the first mutation plus cancellation and repeated Rollback.

Local full-package assembly is provided by `scripts/package-release.ps1`; the
release workflow performs the equivalent isolated packaging jobs in CI and
runs the Windows installer/upgrade smoke on a Windows runner. Portable
artifacts contain compiled binaries and do not require this development
toolchain. A release tag must have a matching `.github/release-notes-<tag>.md`;
publishing deliberately fails when the file is absent.

## Repository map

| Path | Responsibility |
| --- | --- |
| Root `main.go`, `app.go`, `elevation.go`, `rollback.go` and adjacent tests | Wails desktop entry and Go bridge; kept at the root for the existing build contract |
| `cmd/ssh-launchpad/` | CLI wizard, machine JSON entry, terminal and process handling |
| `internal/launchpad/` | Shared planner/executor, probes, platform actions, recovery and engine tests |
| `internal/elevation/` | Shared GUI/CLI elevation request protocol and file permissions |
| `frontend/src/`, `frontend/tests/` | TypeScript/CSS wizard and browser/bridge scenarios |
| `scripts/`, `scripts/internal/` | Bootstrap, build, packaging and offline-pack tooling |
| `packaging/` | Source-controlled launcher and installer templates |
| `profiles/` | Sanitized example profiles only, never real device profiles |
| `tests/` | PowerShell/Pester and release-package smoke tests |
| `.github/` | CI/release workflows and tag-matched release notes |
| `docs/` | Current topic guides and navigation |
| `docs/records/` | Dated candidate audit, UI and acceptance evidence summaries |
| `docs/90.Archive/` | Superseded documents/screenshots, with an ARCHIVE_NOTE per entry |

Tests stay beside Go code when package access is needed. Do not move root Go
files or `frontend/dist` just to make the directory tree more uniform: the
Wails entry embeds `frontend/dist`, and build/packaging commands rely on these
paths. `build/appicon.png` is tracked source, not disposable output.

## Generated and local-only files

| Path | Contents |
| --- | --- |
| `build/bin/`, `build/windows/`, `build/tools/`, `build/test-tmp/` | Build outputs, generated installer files, tools and test scratch data |
| `build/audit/` | Local raw logs, repro fixtures, reports and disposable-host evidence; ignored by repository rules |
| `dist/` | Local assembled release packages |
| `frontend/dist/` | Built UI embedded by Wails; only `.gitkeep` is tracked |
| `frontend/node_modules/`, `frontend/wailsjs/`, `frontend/package.json.md5` | Dependencies, generated bridge bindings and Wails bookkeeping |
| `frontend/test-results/`, `frontend/playwright-report/` | Browser test output and captures |
| `.pi/` | Harness-owned local state; ignored, not project documentation |

Do not clear old acceptance evidence as routine build cleanup. Date and scope
reports so they cannot be mistaken for a current run. Never commit raw host
identities, credentials, exported logs, or journals; commit only sanitized
evidence summaries in `docs/records/`.
