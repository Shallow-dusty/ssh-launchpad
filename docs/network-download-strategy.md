# Network and download strategy

## Source priority

1. Existing system package manager with configured trusted repositories.
2. SSH Launchpad GitHub Release assets.
3. A user-specified HTTPS mirror.
4. A user-specified proxy transporting the same verified artifact.
5. A local offline bundle with its adjacent checksum manifest.
6. A previously downloaded, checksum-verified cache.

Tailscale installation follows the same principle: use a trusted system package
repository or a deliberately supplied offline installer. SSH Launchpad does not
pipe network content into a shell.

## Integrity and availability

- HTTPS is mandatory for network sources.
- SHA-256 is mandatory before installation or extraction.
- The Go downloader supports retry with exponential backoff, `.part` files,
  HTTP range resume, and cache reuse only after validation.
- The bootstrap scripts retry and cache release assets. A hash mismatch aborts
  and leaves the artifact available for diagnosis.
- Proxy and mirror settings are explicit profile or command inputs; environment
  defaults are reported but not turned into trust.
- TLS certificate verification is never disabled.

Checksums prove that an asset matches the published manifest; they do not
replace release signing. `v0.2.0` also publishes an SBOM. Signing and
notarization status is stated in each release.
