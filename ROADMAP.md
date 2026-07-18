# Roadmap

## v0.1.0 — first installable release

Acceptance:

- CLI archives for Windows, Linux, and macOS on amd64 and arm64.
- Standalone PowerShell and POSIX bootstrap bundle.
- At least one successfully built desktop artifact.
- SHA-256 manifest and SBOM.
- CI green for Go, UI, scripts, and package smoke tests.
- Public repository contains no real device profile, token, key, or log.

## v0.2.0 — real target matrix

- Disposable Windows Sandbox/VM Apply and repeat-Apply tests.
- Disposable Ubuntu and macOS Apply and rollback tests.
- Controller-side real SSH handshake and authentication verifier.
- Signed Windows artifacts when certificate infrastructure is available.

## Later

- Signed/notarized macOS distribution.
- Package repositories and managed update channel.
- Organization policy bundles and centrally attested external verification.
