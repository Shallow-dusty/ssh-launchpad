# Release checklist

Run from a clean checkout before tagging:

```text
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend exec playwright install chromium
node scripts/check.mjs release
go test . -run TestReleaseVersionConsistency -count=1
wails build
```

Then:

1. Build release assets with `scripts/package-release.ps1`; build the Windows
   installer via `scripts/build-windows-installer.ps1` when shipping it.
2. Verify every entry in `checksums.txt` and glance at each archive's file
   list for stray files (logs, journals, real profiles, host identities).
3. Update `CHANGELOG.md`, align `frontend/package.json`, `wails.json`,
   `internal/launchpad/types.go`, and script defaults, and add
   `.github/release-notes-<tag>.md` — the release workflow deliberately fails
   before packaging when the notes file is absent.
4. Tag the tested commit, push, wait for a green release workflow, and confirm
   the assets, checksums, and SBOM actually download.

The shared release profile already includes the pinned staticcheck,
govulncheck/gosec scanners, frontend audit, race, documentation and platform
script gates. A passing local profile is not native installer or real-host
acceptance. CI secret scanning remains a separate gate.

Additional acceptance, run when touching the relevant area:

- the mutation interruption matrix: failure before/during/after each action,
  process interruption after intent is journaled, cancellation during Apply,
  repeated Rollback, and GUI elevated Rollback;
- `tests/installer-upgrade-smoke.ps1` after changing the installer;
- a secret scan (e.g. gitleaks) after adding fixtures or sample data.
