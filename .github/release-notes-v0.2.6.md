# SSH Launchpad v0.2.6

Candidate notes; publishing and native acceptance remain separate gates.

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
- Local checks include Go tests/race/vet/staticcheck/govulncheck, ShellCheck,
  frontend typecheck/build, 23 browser scenarios and six-platform cross-builds.
  Windows core tests also passed natively with mocked mutations. Installer,
  upgrade and isolated-VM smoke remain outstanding; no live host Apply was run.
- See `docs/audit-2026-09.md` and `docs/ui-refinement-2026-09.md` for evidence,
  unresolved limits and historical/current documentation distinctions.
