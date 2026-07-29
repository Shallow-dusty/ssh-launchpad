# Development

Requirements: Go 1.25+, Node 22+, pnpm 10+, Wails 2.13, and NSIS for a
Windows installer.

```text
go test ./...
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
pwsh -NoProfile -File scripts/build-windows-installer.ps1 -Version 0.2.3
```

No integration test may change SSH, Tailscale, RDP, or firewall state on a
personal or production host. Use mocks, generated commands, disposable VMs,
Windows Sandbox, and native CI runners.

Release packaging is owned by `scripts/package-release.ps1` and
`.github/workflows/release.yml`. Portable artifacts contain compiled binaries
and do not require this development toolchain. A release tag must have a
matching `.github/release-notes-<tag>.md`; publishing deliberately fails when
the file is absent.
