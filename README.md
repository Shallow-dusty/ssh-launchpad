# SSH Launchpad

SSH Launchpad is a cross-platform SSH bootstrap and recovery studio. It detects
the host, builds a concrete read-only plan, applies the smallest necessary
changes with explicit confirmation, and verifies each layer separately.

It supports three ways to work:

- `ssh-launchpad`: a small Go CLI with structured JSON reports.
- SSH Launchpad Studio: a Wails desktop app backed by the same Go engine.
- `bootstrap.ps1` and `bootstrap.sh`: standalone installers that download,
  verify, install, and optionally run Check, Plan, or Verify without requiring
  the desktop app.

Tailscale is optional. The recommended exposure is tailnet-only, but LAN and
explicit custom CIDRs are supported. Windows and WSL are always treated as
separate targets.

## Safety model

The lifecycle is `Check -> Plan -> Apply -> Verify`.

- Check and Plan are strictly read-only.
- Apply requires `--yes`; high-risk operations are visible as individual
  actions and are journaled.
- Verify does not elevate.
- Active-channel self-cut is blocked by default. Risky service or transport
  restarts can only be scheduled with an external verification target.
- Re-running Apply is idempotent because the planner emits only current diffs.
- Failed Apply attempts can automatically roll back completed reversible
  actions.

No command disables TLS verification. Downloads require HTTPS and a matching
SHA-256 value from the release manifest.

## Quick start

Download the matching archive and `checksums.txt` from the latest release, then
verify it before extraction.

```powershell
.\scripts\bootstrap.ps1 -Version 0.1.0 -Run Check
```

```sh
./scripts/bootstrap.sh --version 0.1.0 --run check
```

Or run the CLI directly:

```text
ssh-launchpad check --profile profiles/example.yaml --output check.json
ssh-launchpad plan --profile profiles/example.yaml --output plan.json
ssh-launchpad apply --profile profiles/example.yaml --yes
ssh-launchpad verify --profile profiles/example.yaml --output verify.json
ssh-launchpad rollback --journal <journal.json>
```

Never put a private key in a profile. Public release examples contain
placeholders only.

## What is verified

The report separates:

1. client and server installation;
2. SSH configuration syntax;
3. service status and startup policy;
4. firewall port and source scope;
5. optional Tailscale state;
6. listener reachability;
7. SSH protocol/KEX;
8. authentication;
9. the remote security token/identity.

A reachable TCP port is not reported as successful authentication.

## Build

Requirements: Go 1.25+, Node 22+, pnpm 10+, and Wails 2.13 for the desktop app.

```text
go test ./...
cd frontend
pnpm install
pnpm run build
pnpm run test:e2e
cd ..
wails build
```

Windows installers use NSIS. `v0.1.0` publishes the desktop installer for
Windows amd64; Linux and macOS desktop packaging remains a later native-runner
milestone. CLI archives are published for all listed OS/architecture targets.
Artifacts are unsigned and unnotarized.

## Documentation

- [Architecture](docs/architecture.md)
- [Platform support](docs/platform-support.md)
- [Network and download strategy](docs/network-download-strategy.md)
- [Threat model](docs/threat-model.md)
- [Troubleshooting and recovery](docs/troubleshooting.md)
- [Release verification](docs/release-verification.md)
- [Security policy](SECURITY.md)

The project is licensed under the MIT License.
