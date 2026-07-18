# Security policy

## Supported versions

Security fixes are provided for the latest published minor release.

## Reporting

Do not open a public issue for a vulnerability that could expose credentials,
weaken remote access, bypass confirmation, or interrupt a control channel.
Use GitHub's private vulnerability reporting feature for this repository.

Include the affected version, platform, minimal reproduction, impact, and any
known workaround. Do not attach real private keys, tokens, or unredacted logs.

## Security guarantees

Check and Plan are read-only. Verify does not elevate. Apply requires explicit
confirmation and blocks control-channel self-cut by default. Network downloads
require HTTPS and SHA-256 verification.

These guarantees are bugs if violated and should be reported privately.
