# Network and download strategy

## Source priority

1. Existing system package manager with configured trusted repositories.
2. SSH Launchpad GitHub Release assets.
3. A user-specified HTTPS mirror.
4. A user-specified proxy transporting the same verified artifact.
5. A local offline bundle with an explicitly pinned SHA-256.
6. A previously downloaded, checksum-verified cache.

Tailscale installation follows the same principle: use a trusted system package
repository or a deliberately supplied offline installer. SSH Launchpad does not
pipe network content into a shell. Mirror/proxy/cache selection for downloading
SSH Launchpad itself belongs to the bootstrap scripts' command-line options;
they do not consume a profile's `download` fields. Those profile fields are
reserved for dependency adapters. Currently these adapters support only the
configured system package manager and, on Windows Tailscale, a pinned offline
installer. Unsupported combinations fail in Plan instead of silently falling
back to the network.

## Integrity and availability

- HTTPS is mandatory for network sources.
- SHA-256 is mandatory before installation or extraction. Offline executables
  are copied into privileged staging while hashing, and only those staged bytes
  are executed.
- The Go downloader supports retry with exponential backoff, `.part` files,
  HTTP range resume, and cache reuse only after validation.
- The bootstrap scripts retry and cache release assets. A hash mismatch aborts
  and leaves the artifact available for diagnosis.
- Bootstrap proxy and mirror settings are explicit command inputs; environment
  defaults are reported but not turned into trust. Profile settings are not
  implicitly forwarded to package managers.
- TLS certificate verification is never disabled.

Checksums prove that an asset matches the published manifest; they do not
replace release signing. `v0.2.0` also publishes an SBOM. Signing and
notarization status is stated in each release.
