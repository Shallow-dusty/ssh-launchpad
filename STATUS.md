# Current status

Real-host acceptance baseline: **2026-09-16**. Documentation reorganized and
code-only local regression/security scans refreshed on **2026-10-09**. This does not renew
real-host or remote CI evidence.

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
| September local checks | Serialized Go unit/race tests, vet, staticcheck, govulncheck, ShellCheck, frontend typecheck/build, and 23 browser scenarios passed | Historical baseline; fresh October checks are recorded separately below |
| Platform builds | Cross-builds plus Windows-native core tests with mocked mutations passed | Not live service or GUI/UAC acceptance |
| Disposable Windows Server 2022 | OpenSSH install, firewall scoping, Tailscale join, real key-only SSH login, port changes, self-cut refusals, rollbacks, and v0.2.5 → v0.2.6 installer lifecycle passed | One server environment, not Windows 10/11 |
| Field fixes | Ten discovered defects fixed with regression coverage | See the acceptance record for the exact observed scope |
| Uninstaller refusal | Missing install marker now returns exit 68; metadata regression test covers the change | Fixed after the live round; live recheck is pending |

Detailed evidence, not duplicate status summaries:

- [Security and recovery audit, 2026-09-13](docs/records/audit-2026-09.md)
- [Real-machine acceptance, 2026-09-15/16](docs/records/acceptance-2026-09.md)
- [UI refinement and browser evidence, 2026-09-13](docs/records/ui-refinement-2026-09.md)
- [Version changes](CHANGELOG.md)

## Code consolidation — 2026-10-08/09

- Frontend composition, backend adapter, wizard, transfers, advanced controls
  and feedback are separate modules. Desktop errors never fall through to
  preview success; native key validation remains authoritative.
- Desktop/CLI Go files and engine probe/action/journal responsibilities were
  split within the existing packages. Of 161 affected declarations, 153 remain
  identical after formatting; eight reviewed changes retain structured failure
  reports/reasons. Planner, platform commands, probes and journal declarations
  are unchanged; existing JSON fields, schema v1 and exit codes remain stable.
- Go now generates TypeScript wire models, App signatures and profile defaults.
  Failure reason codes distinguish shared exit codes without parsing error prose.
  Local and CI quick/full/release gates share one runner and scanner-pin source.
- Final release-profile checks on Go 1.26.9 passed: serialized unit/race tests,
  vet, typecheck/build, **35 browser scenarios**, 11 Node check/document tests,
  staticcheck, govulncheck and gosec's high-severity gate.
- **32 Windows Pester cases**, five Windows-native bridge/contract tests,
  Windows desktop test compilation and all six CLI target builds passed.
  Pester covers UNC offline-pack paths and escaping/rooted payload rejection.
  Details: [dated consolidation evidence](docs/records/code-consolidation-2026-10-09.md).
- No cloud VM, live Apply, installer lifecycle, new remote CI run or system
  mutation acceptance was performed. Source responsibilities are in
  [development](docs/development.md#source-responsibilities).

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

The October rescan of Go 1.25.13 reported ten reachable standard-library
advisories requiring Go 1.26.9; the earlier September clean scan is historical.
The user-approved upgrade now requires **Go 1.26.9** in `go.mod` and updates
`golang.org/x/crypto` to **v0.56.0** for GO-2026-6354/6355. The final
post-upgrade govulncheck reports **0 reachable vulnerabilities**, no advisories
in imported packages, and one module-only advisory.

The separate GO-2026-5932 (unmaintained openpgp) use review is complete: the
current production dependency graph contains no openpgp package. Its upstream
module advisory remains; it is not described as fixed by the SDK upgrade.

## Where to go next

- **Next work and priorities:** [ROADMAP.md](ROADMAP.md)
- **Build/test commands and directory map:** [development](docs/development.md)
- **Before tagging:** [release checklist](docs/release-verification.md)
- **All documentation:** [docs/README.md](docs/README.md)

Completed version narratives live in CHANGELOG, dated evidence in
`docs/records/`, and superseded material in `docs/90.Archive/`. This file
contains only the current baseline and open boundaries.
