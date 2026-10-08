# Current status

Product validation baseline: **2026-09-16**. Documentation reorganized:
2026-10-08; this does not renew product test or release evidence.

## Release state

| Item | Recorded state |
| --- | --- |
| Published release | [v0.2.5](https://github.com/Shallow-dusty/ssh-launchpad/releases/tag/v0.2.5) |
| Source version | **v0.2.6 candidate**, not tagged or published in this baseline |
| Last recorded green CI | Run `34736001130`, commit `68ee28f`; all eight jobs passed. This is not proof of CI on later commits. |
| Real-machine acceptance | One disposable Azure Windows Server 2022 round, 2026-09-15/16; details below |

## What the product does

- Beginner-first Chinese/English Windows desktop wizard and a matching CLI.
- Shared Go Check / Plan / Apply / Verify / Rollback engine for Windows,
  Linux, macOS, and a distinct WSL target layer.
- Controller public-key onboarding, optional provisioning cards and Tailscale
  bootstrap; private keys stay on the controller.
- Tailnet-first exposure, scoped firewall plans, self-cut protection,
  digest-bound confirmation, mutation locks, and recovery journals.
- Windows installer, multi-platform portable CLI bundles, bilingual launchers,
  offline help, and checksummed dependency-pack builders.

Check and Plan are read-only; Verify never elevates. The UI does not decide
safety policy or build system commands. See [architecture](docs/architecture.md)
and [platform support](docs/platform-support.md) for the execution contract.

## v0.2.6 candidate evidence

| Area | Recorded result | Limit |
| --- | --- | --- |
| Local checks | Serialized Go unit/race tests, vet, staticcheck, govulncheck, ShellCheck, frontend typecheck/build, and 23 browser scenarios passed | Historical checks, not rerun by reorganizing documentation |
| Platform builds | Cross-builds plus Windows-native core tests with mocked mutations passed | Not live service or GUI/UAC acceptance |
| Disposable Windows Server 2022 | OpenSSH install, firewall scoping, Tailscale join, real key-only SSH login, port changes, self-cut refusals, rollbacks, and v0.2.5 → v0.2.6 installer lifecycle passed | One server environment, not Windows 10/11 |
| Field fixes | Ten discovered defects fixed with regression coverage | See the acceptance record for the exact observed scope |
| Uninstaller refusal | Missing install marker now returns exit 68; metadata regression test covers the change | Fixed after the live round; live recheck is pending |

Detailed evidence, not duplicate status summaries:

- [Security and recovery audit, 2026-09-13](docs/records/audit-2026-09.md)
- [Real-machine acceptance, 2026-09-15/16](docs/records/acceptance-2026-09.md)
- [UI refinement and browser evidence, 2026-09-13](docs/records/ui-refinement-2026-09.md)
- [Version changes](CHANGELOG.md)

## Open validation and product limits

- Unelevated Windows Apply must still be verified end to end: exit 4
  (`NeedsElevation`) with no changes. The first host's Administrator token
  could not exercise this gate.
- Windows 10/11, actual GUI/UAC click-through, and WebView2-less hosts remain
  uncovered. Linux/macOS system-changing paths have generated-command and
  fixture coverage, not real-host acceptance.
- Winget-less Windows hosts cannot use one global download strategy for both
  OpenSSH capability installation and offline Tailscale installation; the first
  round used two stages.
- UFW custom raw rules are not inventoried. Third-party broad firewall rules
  can admit access beyond this tool's managed scopes; the plan warns.
- Rollback is fail-fast and only covers reversible actions. A failed recovery
  stops the chain and requires journal review; software installation and
  Tailnet membership may remain.
- Windows artifacts are unsigned; macOS artifacts are not notarized.
- No system-changing acceptance was run on the development workstation or a
  personal production host. The disposable Azure environment was destroyed;
  cleanup is recorded in the acceptance report.

## Dependency follow-up

`golang.org/x/crypto` is v0.55.0 on Go 1.25.13. The recorded govulncheck result
has no reachable vulnerabilities, but module advisories GO-2026-6354/6355
(channel-deadlock DoS) and GO-2026-5932 (unmaintained openpgp) remain. Clearing
the channel-deadlock advisories requires x/crypto v0.56.0 and Go 1.26; the
openpgp maintenance advisory is a separate dependency-use review, not a claim
that a toolchain upgrade fixes it.

## Where to go next

- **Next work and priorities:** [ROADMAP.md](ROADMAP.md)
- **Build/test commands and directory map:** [development](docs/development.md)
- **Before tagging:** [release checklist](docs/release-verification.md)
- **All documentation:** [docs/README.md](docs/README.md)

Completed version narratives live in CHANGELOG, dated evidence in
`docs/records/`, and superseded material in `docs/90.Archive/`. This file
contains only the current baseline and open boundaries.
