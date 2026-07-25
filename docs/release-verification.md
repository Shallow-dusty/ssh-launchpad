# Release verification

Run from a clean checkout:

```text
go test ./...
go test -race ./...
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go run github.com/securego/gosec/v2/cmd/gosec@latest -exclude-generated -severity high ./...
shellcheck scripts/bootstrap.sh scripts/new-offline-pack.sh "packaging/launchers/Start SSH Launchpad.command"
pwsh -NoProfile -Command "Invoke-Pester -Path tests -CI"
cd frontend
pnpm install --frozen-lockfile
pnpm audit --audit-level high
pnpm run typecheck
pnpm run build
pnpm run test:e2e
cd ..
wails build
```

Then:

1. Build release assets with `scripts/package-release.ps1`.
2. On a clean disposable/current-user install state, run
   `tests/installer-upgrade-smoke.ps1` with the previous released installer and
   the new installer. It must preserve the install directory and sentinel file,
   keep one uninstall entry and one shortcut set, update both Windows version
   resources, and cleanly uninstall afterward.
3. Run `tests/package-smoke.ps1` against the staging directory.
4. Verify every entry in `checksums.txt`.
5. Inspect each archive's file list.
6. Run a secret scanner over Git history and the unpacked staging directory.
7. Search artifacts for hostnames, IP addresses, usernames, private key
   markers, cookies, tokens, logs, journals, and non-example profiles.
8. Review dependency licenses and attach the generated SBOM.
9. Confirm `CHANGELOG.md` and the tag-matched
   `.github/release-notes-<tag>.md` state signing/notarization limits; run the
   release-metadata Pester contract.
10. Tag only the exact tested commit.

Publishing a tag is not sufficient: the release is complete only when assets,
checksums, and SBOM are downloadable and the release workflow is green.
