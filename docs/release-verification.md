# Release verification

Run from a clean checkout:

```text
go test ./...
go vet ./...
shellcheck scripts/bootstrap.sh
pwsh -NoProfile -Command "Invoke-Pester tests/bootstrap.Tests.ps1"
cd frontend
pnpm install --frozen-lockfile
pnpm run typecheck
pnpm run build
pnpm run test:e2e
cd ..
wails build
```

Then:

1. Build release assets with `scripts/package-release.ps1`.
2. Run `tests/package-smoke.ps1` against the staging directory.
3. Verify every entry in `checksums.txt`.
4. Inspect each archive's file list.
5. Run a secret scanner over Git history and the unpacked staging directory.
6. Search artifacts for hostnames, IP addresses, usernames, private key
   markers, cookies, tokens, logs, journals, and non-example profiles.
7. Review dependency licenses and attach the generated SBOM.
8. Confirm `CHANGELOG.md` and release notes state signing/notarization limits.
9. Tag only the exact tested commit.

Publishing a tag is not sufficient: the release is complete only when assets,
checksums, and SBOM are downloadable and the release workflow is green.
