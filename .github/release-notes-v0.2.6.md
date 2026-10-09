# SSH Launchpad v0.2.6

Candidate notes; publishing remains a separate gate.

## Code consolidation and security refresh (2026-10-08/09)

- Split frontend controllers/backend access and Go entry/probe/action/journal
  files by responsibility, within existing packages and without a UI rewrite.
- Generate frontend wire types, App signatures and defaults from Go; retain
  structured failures with optional reason codes instead of parsing error prose.
- Share local/CI check gates, scanner pins and documentation validation.
- Require Go 1.26.9, update x/crypto to v0.56.0 and transitive source-map-js to
  1.2.2. Fresh scans report no reachable Go vulnerabilities or frontend advisories.
  The unused openpgp module advisory remains documented, not hidden.
- Final local checks include 35 browser scenarios, 32 Windows Pester cases,
  Windows-native bridge/contract tests and six CLI target builds. No new cloud,
  live GUI/UAC or installer acceptance was performed; see
  `docs/records/code-consolidation-2026-10-09.md`.

## Field acceptance fixes

First acceptance round on a disposable Windows Server 2022 host (details in
`docs/records/acceptance-2026-09.md`) — highlights:

- A freshly provisioned Windows host with no `sshd_config` is now a recognised
  intermediate state: the wizard warns, writes the packaged stock template, and
  creates the configuration directory instead of dead-ending.
- Missing SSH host keys are generated with SYSTEM/Administrators-only ACLs and
  SYSTEM ownership, so `sshd -t` and the first start succeed.
- Firewall planning tells apart the tool's own managed rule, stale managed
  rules from an earlier port (disabled with rollback state), third-party broad
  rules such as Tailscale's `Tailscale-In` (warning), and Windows rules bound to
  unrelated programs, app containers or services.
- Rollbacks consume their backup markers, tolerate already-absent resources,
  and no longer report success as `rollback-failed`, which previously aborted
  the remaining recovery actions.
- Generated-command syntax coverage now includes every planned Windows action.
- A silent uninstall that refuses to delete files because the install marker is
  missing now exits with code 68 instead of 0.

## Security and recovery fixes

- Check now fails closed when SSH configuration contains unsupported `Match`,
  `ListenAddress`, included `Port`, or recursive/unsupported `Include` policy.
- Changing the SSH port removes prior top-level `Port` directives and Verify
  considers every effective port, so the old listening port is not left behind.
- Windows firewall probing uses the ActiveStore policy, includes Any-protocol
  inbound allows, requires all profiles to be enabled with non-Allow default
  inbound action, and fails closed on unreadable inventory.
- UFW probing uses `ufw status verbose`, requires a deny/reject incoming
  default, and flags unsupported visible inbound syntax and relevant port
  ranges. Custom before/after raw rules or independent packet-filter rules
  are not inventoried; these hosts still require manual review.
- firewalld probing supports only an exactly-known single active default zone,
  rejects services/policies/direct rules/drift and non-accept rich rules, and
  never treats reject/drop rules as successful allow rules.
- Firewall mutations propagate every command failure and rollback removes only
  scopes added by the reviewed plan. Windows managed firewall rules preserve
  prior scopes and restore disabled conflicting rules.
- Apply now writes an in-flight journal entry before the first possible side
  effect. A failed or interrupted reversible action is included in recovery,
  while old journals without write-ahead intent fail closed for manual review.
- Automatic recovery uses a bounded recovery context independent of the
  cancelled Apply context. Service rollback restores prior running and startup
  state; authorized_keys and firewall recovery preserve original metadata.
- All desktop, CLI, and elevated helpers share a system mutation lock across
  re-plan, Apply, and Rollback to prevent concurrent config/firewall changes.
- SSH dependency installation is explicitly phased: a fresh Check/Plan is
  required after packages are restored, so missing authorized-key evidence is
  never treated as safe. Missing Windows sshd.exe repair now remains reachable.
- The GUI keeps failed Apply reports and journal paths available for recovery;
  Rollback can run through the same digest-bound elevated helper protocol.
- The GUI status deadline is no longer mistaken for cancellation: it continues
  tracking the helper and blocks conflicting retries until a terminal result.

## Wizard interaction

- Explain the read-only check, review and connection handoff on the home page.
- Show honest pending states, irreversible actions and per-action failure results.
- Offer recovery/report export directly on failures; retry goes through fresh review.
- Copy key-free connection details and distinguish local checks from controller
  authentication. Dark/narrow layouts and keyboard focus are covered by tests.
- Risky delays now stay in process with the mutation lock held. Legacy detached
  tasks require manual verification before recovery.

## Build and release

- Release defaults, Wails metadata, frontend metadata, bootstrap scripts, and
  tag-matched notes are validated before release packaging.
- The September local baseline included Go tests/race/vet/staticcheck/govulncheck,
  ShellCheck, frontend typecheck/build, 23 browser scenarios and six-platform cross-builds.
  Windows core tests also passed natively with mocked mutations. A subsequent
  disposable Windows Server 2022 round accepted live Apply/Verify/Rollback and
  installer upgrade/uninstall. Windows 10/11, GUI/UAC, the unelevated Apply gate
  and the post-round uninstall refusal exit-code fix still need live acceptance.
- See `docs/records/audit-2026-09.md`, `docs/records/acceptance-2026-09.md`
  and `docs/records/ui-refinement-2026-09.md` for evidence,
  unresolved limits and historical/current documentation distinctions.
