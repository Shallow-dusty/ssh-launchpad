# Offline dependency packs

SSH Launchpad itself is fully portable and can run Check, Plan, Verify, and
configuration logic without Go, Node, Wails, or an internet connection.

When the target does not already have OpenSSH or optional Tailscale, a fully
offline Apply also needs the platform installers or packages. They are not
bundled in source or the standard Release because redistribution terms and
platform versions differ.

Create a local pack with `scripts/new-offline-pack.ps1` or
`scripts/new-offline-pack.sh`. The metadata input is:

```json
{
  "schemaVersion": 1,
  "components": [
    {
      "file": "OpenSSH-installer.msi",
      "sourceUrl": "https://vendor.example/download",
      "license": "SPDX-or-vendor-license-name",
      "redistributionAllowed": false
    }
  ]
}
```

The command copies only explicitly listed local files and writes a
`manifest.json` containing source URL, license declaration, and redistribution
flag plus `bundle-checksums.txt` containing every payload SHA-256 (the
PowerShell packer also repeats the SHA-256 in the manifest). It rejects
non-HTTPS source metadata. A pack with
`redistributionAllowed: false` is for the creating user's local transfer only
and must not be uploaded to a Release.

In v0.2.0 the pack is a verified transport container. Extract it locally and
select the required installer as `download.offlineBundle`; automatic selection
of multiple third-party components is intentionally deferred.
