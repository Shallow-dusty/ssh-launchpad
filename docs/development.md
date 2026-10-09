# Development

Requirements: Go 1.26.9+ (the security minimum in `go.mod`), Node 22+ (CI uses 24), pnpm 10+, and installed
Playwright Chromium. Full checks also require ShellCheck on Unix or Pester
5.7.1+ with PowerShell 7 on Windows. Wails 2.13/NSIS are needed only for
native desktop/installer builds.

```text
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend exec playwright install chromium
node scripts/check.mjs quick
node scripts/check.mjs full
# Full checks plus the pinned Go security scanners:
node scripts/check.mjs release
```

The same runner is used by CI and release workflows. Every workflow Go setup
reads `go.mod`, not a separate version pin; its patch-level `go` directive
also enforces the security minimum for local builds. `quick` covers Go
unit/vet/CLI build, generated types and documentation; `full` adds race tests,
frontend dependency audit/build/browser tests and platform script checks.
`release` adds staticcheck, govulncheck and gosec. None of these commands runs
live Apply, installer lifecycle acceptance, tagging or publication.

Individual groups: `go`, `race`, `types`, `ui`, `docs`, `scripts`, `vuln`,
`security`. Scanner pins live only in `scripts/check.mjs`; failing groups stop
the run. Gosec excludes ignored/generated `build/` artifacts, not source
packages; all production packages and maintained Go tools stay in scope. `ui` builds once, via the existing `test:e2e` script.

Build the versioned per-user Windows upgrade installer through the project
wrapper so the custom NSIS identity/upgrade checks and GUI version resources
are applied:

```text
pwsh -NoProfile -File scripts/build-windows-installer.ps1 -Version 0.2.6
```

On Windows, `node scripts/check.mjs scripts` invokes
`scripts/check-pester.ps1` in a child PowerShell process. Its XML output stays
in `build/audit/pester/testResults.xml`; a failed suite propagates to the
runner instead of being hidden by a later successful command.

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
| Root `main.go`, `app*.go`, `elevation.go`, `rollback.go` and adjacent tests | Wails desktop entry and Go bridge; kept at the root for the existing build contract |
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

### Source responsibilities

Frontend modules keep the existing vanilla TypeScript UI and a single shared
state, without a framework migration:

| Module | Responsibility |
| --- | --- |
| `frontend/src/main.ts` | Composition, application shell, rendering and DOM event wiring |
| `state.ts`, `profile.ts` | State construction, shared defaults and profile normalization |
| `backend.ts` | The sole Wails App access point; binds desktop or preview at startup, owns file-dialog/download and mock differences |
| `wizard.ts` | Check/review/install/verify transitions, key-input debounce, job polling and recovery |
| `transfers.ts` | Public-key/profile/card workflows and browser file parsing |
| `advanced.ts` | Live advanced settings, report export and update feedback |
| `feedback.ts` | Dialogs, toast/announcements, error presentation and clipboard handoff |
| `controller-context.ts` | Explicit shared collaborators passed to controllers; no service container or hidden global lookup |
| `views.ts` | Existing HTML renderers/display models; null Go action lists are treated as empty, without layout changes |

### Go/TypeScript wire contract

`scripts/internal/bridge-types` derives JSON-tagged models, enum values, App
method signatures and default profile values from Go. `pnpm run dev`,
`typecheck`, and `build` generate `build/contracts/wire-types.ts` first; this
ignored build artifact is never hand-edited or committed. No generator
framework or new package dependency is introduced.

`frontend/src/types.ts` contains only UI refinements: normalized non-null
profile arrays/maps, form strings and the user-facing stage subset. Models
and Wails signatures are no longer duplicated there. Mock reports include
the same wire metadata; the adapter normalizes Go's nullable profile values.
The Vite dev server permits only frontend files and the contracts directory,
not sibling raw audit/test artifacts.

Desktop Go methods remain on the same `App` type: `app.go` owns engine entry
points, `app_jobs.go` job lifecycle, `app_keys.go` controller-key operations,
and `app_exports.go` profile/card/report I/O. CLI dispatch remains in `main.go`,
with its wizard, key operations, output and language helpers in adjacent files.

Inside `internal/launchpad`, `probe.go` coordinates checks; `ssh_probe.go` and
`firewall_probe*.go` own detailed evidence. `planner.go` retains decision policy;
`install_actions.go`, `service_actions.go`, and the existing config/key/firewall
action files construct commands. `executor.go` owns execution/recovery and
`journal.go` journal persistence. These are file boundaries, not new Go packages:
command construction stays available across platforms for generated-command tests.

Tests stay beside Go code when package access is needed. Do not move root Go
files or `frontend/dist` just to make the directory tree more uniform: the
Wails entry embeds `frontend/dist`, and build/packaging commands rely on these
paths. `build/appicon.png` is tracked source, not disposable output.

## Generated and local-only files

| Path | Contents |
| --- | --- |
| `build/bin/`, `build/windows/`, `build/tools/`, `build/test-tmp/` | Build outputs, generated installer files, tools and test scratch data |
| `build/audit/` | Local raw logs, repro fixtures, reports and disposable-host evidence; ignored by repository rules |
| `build/contracts/` | Go-derived TypeScript wire shapes, bridge signatures and defaults; generated before frontend checks |
| `dist/` | Local assembled release packages |
| `frontend/dist/` | Built UI embedded by Wails; only `.gitkeep` is tracked |
| `frontend/node_modules/`, `frontend/wailsjs/`, `frontend/package.json.md5` | Dependencies, generated bridge bindings and Wails bookkeeping |
| `frontend/test-results/`, `frontend/playwright-report/` | Browser test output and captures |
| `.pi/` | Harness-owned local state; ignored, not project documentation |

Do not clear old acceptance evidence as routine build cleanup. Date and scope
reports so they cannot be mistaken for a current run. Never commit raw host
identities, credentials, exported logs, or journals; commit only sanitized
evidence summaries in `docs/records/`.
