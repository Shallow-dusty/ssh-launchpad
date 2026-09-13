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
