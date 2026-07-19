# Development

Requirements: Go 1.25+, Node 22+, pnpm 10+, Wails 2.13, and NSIS for a
Windows installer.

```text
go test ./...
go vet ./...
cd frontend
pnpm install --frozen-lockfile
pnpm run build
pnpm run test:e2e
cd ..
wails build
```

No integration test may change SSH, Tailscale, RDP, or firewall state on a
personal or production host. Use mocks, generated commands, disposable VMs,
Windows Sandbox, and native CI runners.

Release packaging is owned by `scripts/package-release.ps1` and
`.github/workflows/release.yml`. Portable artifacts contain compiled binaries
and do not require this development toolchain.
