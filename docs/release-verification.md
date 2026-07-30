# Release checklist

Run from a clean checkout before tagging:

```text
go test ./...
go vet ./...
cd frontend
pnpm install --frozen-lockfile
pnpm run typecheck
pnpm run build
pnpm run test:e2e
cd ..
wails build
```

Then:

1. Build release assets with `scripts/package-release.ps1`; build the Windows
   installer via `scripts/build-windows-installer.ps1` when shipping it.
2. Verify every entry in `checksums.txt` and glance at each archive's file
   list for stray files (logs, journals, real profiles, host identities).
3. Update `CHANGELOG.md` and add `.github/release-notes-<tag>.md` — the
   release workflow deliberately fails when the notes file is absent.
4. Tag the tested commit, push, wait for a green release workflow, and confirm
   the assets, checksums, and SBOM actually download.

Optional hardening, run when touching the relevant area:

- `staticcheck`, `govulncheck`, `gosec -severity high`;
- `shellcheck` on the POSIX scripts and macOS launcher;
- Pester under `tests/` (Windows);
- `tests/installer-upgrade-smoke.ps1` after changing the installer;
- a secret scan (e.g. gitleaks) after adding fixtures or sample data.
