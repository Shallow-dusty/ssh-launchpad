# Code consolidation and security refresh — 2026-10-09

Work started 2026-10-08; final local verification completed 2026-10-09.
Source baseline before consolidation: `ffc9091`. Source remains the v0.2.6
candidate; this record does not claim publication or a new remote CI run.

## Scope and implementation

- Split frontend composition, backend access, state/profile normalization,
  wizard, transfers, advanced controls and feedback without changing framework
  or layout. `backend.ts` is the only Wails access point; desktop failures never
  fall through to preview success.
- Split desktop/CLI entry files and engine probes, action builders and journal
  persistence within existing Go packages. An AST comparison found 153 of 161
  affected declarations unchanged after formatting. Exactly eight reviewed
  failure-contract changes and four additions account for the remainder.
  Planner, platform command, probe and journal declarations are unchanged.
- Generate TypeScript JSON models, App method signatures, enums and profile
  defaults from Go into ignored `build/contracts/wire-types.ts`. UI refinements
  normalize nullable values instead of maintaining another wire schema.
- Add optional report `reasonCode`, retaining schema v1 and existing exit codes.
  GUI/CLI use metadata, not English error words. Wails expected failures retain
  reports, and Verify drift retains inspectable remaining evidence.
- Unify local/CI check profiles with one scanner-pin source, fail-fast behavior,
  documentation/anchor validation and Pester output under `build/audit/`.
  Gosec excludes only generated/ignored `build/`, which holds standalone repro
  scripts rather than production packages. All six production/tool packages
  remain in its scan.
- Fix offline-pack UNC filesystem paths: remove PowerShell provider prefixes,
  canonicalize parent segments before .NET access/containment checks, and retain
  rooted/traversal rejection. Positive packaged/UNC and negative path tests pass.

## Security refresh

The fresh audit first found GHSA-68fv-2mgg-jv7q in transitive source-map-js
1.2.1. Updating the lockfile to compatible 1.2.2 cleared the frontend audit.
The first govulncheck rescan of Go 1.25.13 then reported ten reachable
standard-library advisories with fixes in Go 1.26.9.

The user explicitly approved extending this round to the SDK migration:
`go.mod` now requires Go 1.26.9 and x/crypto v0.56.0. Its patch-level `go`
directive is the local/CI security minimum; every workflow Go setup reads
that file, with no second version pin. The redundant toolchain directive was
removed by Go tooling. No global Go or proxy configuration was changed.

Post-upgrade govulncheck reports zero reachable vulnerabilities and zero
advisories in imported packages. GO-2026-5932, the unmaintained openpgp package,
remains a module-only advisory: a verbose scan and the current `go list -deps
./...` production graph confirm that openpgp is not used. This is a completed
dependency-use review, not a claim that the package advisory itself is fixed.

## Final local results

| Check | Result |
| --- | --- |
| `node scripts/check.mjs release` on Go 1.26.9 | Passed, including serialized unit/race tests, vet and CLI build |
| Generated wire types / TypeScript / production frontend build | Passed |
| Playwright | 35 scenarios passed, including actual dev-server filesystem boundaries |
| Shared runner and documentation unit tests | 11 passed |
| staticcheck / govulncheck / gosec high-severity gate | Passed; 0 reachable vulnerabilities / 0 high-severity issues |
| Windows desktop test compilation | Passed |
| Windows-native new bridge/contract tests | Five passed, using fixture engines, without live mutations |
| Windows-native Pester | 32 passed, including UNC offline-pack creation and escaping/rooted payload rejection |
| Windows/Linux/macOS CLI, amd64 and arm64 | All six targets compiled |
| AST invariant comparison | Passed: 153 unchanged declarations plus the exact reviewed change set |
| Final documentation links/anchors | 96 targets/headings passed; only two documented verbatim snapshots excluded |
| Staged-diff Gitleaks scan | Passed, no leaks; raw report retained under ignored audit output |

Windows tests used a process-only execution-policy parameter for authored
WSL-local test scripts; machine policy and elevation were not changed. Module
fetches used a temporary HTTPS Go mirror after route failures; npm retained
its native connection. These personal execution choices are not project
configuration requirements.

Raw success/failure logs and dependency graphs are retained in ignored
`build/audit/code-consolidation-20261008/`; Pester XML is in
`build/audit/pester/`, browser artifacts in `frontend/test-results/`. Previous
September acceptance evidence remains intact.

No VM creation, real Apply, SSH/firewall/service mutation, GUI/UAC acceptance,
installer lifecycle acceptance, tag, push or publication was performed. Open
real-host boundaries remain in [STATUS](../../STATUS.md) and
[ROADMAP](../../ROADMAP.md).
