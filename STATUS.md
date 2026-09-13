# Status

Last locally verified: 2026-09-13

Current release: [`v0.2.6`](https://github.com/Shallow-dusty/ssh-launchpad/releases/tag/v0.2.6) (candidate; release CI pending)

## v0.2.6 (2026-09-05, from OneClick field lessons)

Security and recovery hardening after the 2026-09-12 full audit:

- SSH policy checks fail closed on unsupported `Match`, custom `ListenAddress`,
  included `Port`, recursive `Include`, and extra effective SSH ports; port
  changes remove prior top-level `Port` directives instead of leaving the old
  listener configured.
- Windows/UFW/firewalld reject unsupported evidence in their supported
  inventories: Any-protocol Windows rules, UFW application
  profiles, firewalld services/zones/direct rules, runtime drift, and
  non-accept rules are no longer guessed as safe allow rules. UFW custom raw
  rules outside `status verbose` remain an explicit coverage gap; see
  `docs/audit-2026-09.md`.
- Firewall mutations propagate every command failure. Unix rollback removes
  only scopes introduced by the reviewed plan; Windows rollback restores prior
  managed-rule scopes and conflicting-rule enabled state.
- Apply journals in-flight intent before the first possible side effect.
  Failed/interrupted reversible actions are recoverable, legacy uncertain
  journals fail closed for manual review, automatic recovery uses a bounded
  context independent of Apply cancellation, and all entry points share a
  system mutation lock.
- Service and authorized_keys rollback restores prior running/startup state,
  file contents, ownership, and Windows ACLs. Missing Windows sshd.exe repair
  is a deliberate first phase followed by a fresh Check/Plan.
- The GUI retains failed Apply reports for recovery, supports digest-bound
  elevated Rollback on Windows, keeps tracking an over-deadline helper instead
  of declaring it cancelled, and blocks navigation during busy mutations.
- Release defaults and candidate notes are aligned through a pre-packaging
  metadata gate.
- Risky delays retain the mutation lock and are cancellable; no detached
  mutation is left behind. Old scheduled journals require manual recovery.
- UI refinement retains the existing three-step framework: explicit flow,
  recovery actions on failures, fresh review for incomplete verification,
  shareable connection details and a clear local-check boundary. See
  `docs/ui-refinement-2026-09.md` for reference and viewport evidence.
- Final local checks: serialized Go package tests with race detection, vet,
  staticcheck, govulncheck, ShellCheck, frontend typecheck/build, and 23 browser
  scenarios passed. Windows launchpad tests (including generated PowerShell
  syntax) also passed natively through WSL interop. Windows amd64 and macOS
  arm64 builds passed; this is not live system-change acceptance.

Earlier v0.2.6 fixes:

- Windows firewall scope comparison normalizes netmask-form scopes
  (`100.64.0.0/255.192.0.0` → `100.64.0.0/10`); before this, a correctly
  scoped rule reported drift on every Check and was rebuilt on every Apply.
- Windows OpenSSH Server self-repair: when the sshd service is registered
  but sshd.exe is missing (antivirus quarantine), the plan now offers a
  capability reinstall instead of dead-ending in failed sshd probes. Gated
  to Windows-capability installs; foreign installers fail with explicit
  guidance. Validated end-to-end against the same failure mode in the
  [OneClick minimal product line](https://github.com/Shallow-dusty/remote-onboarder).
- authorized_keys merge commands survive PowerShell 5.1 single-element
  pipeline unrolling (explicit array wrapping).
- Check models Win32-OpenSSH's `administrators_authorized_keys` redirection
  for admin-group users when `sshd -T` prints the stock per-user default.

## v0.2.5

- Desktop wizard rebuilt around user tasks (check → review → finish; repair
  mode: diagnose → repair → verify) following the deep audit in
  `docs/design-audit-2026-08.md`: self-driving plan step with preselected
  keys, consequence-labelled network choice, issue lists, persistent error
  states, live-applied advanced settings, restrained visual tokens, and
  Lucide icons. Safety simplifications: rollback-journal digest mismatch
  downgraded to a warning; the GUI's third pre-elevation Probe+Plan removed
  (digest check stays authoritative inside Apply).
- Local candidate validation covers Go unit/race/vet, staticcheck,
  govulncheck, gosec, shellcheck, and gitleaks, Windows/macOS
  cross-compilation, frontend typecheck/build, and 14 browser scenarios.
  Windows-native checks (Pester, Wails/NSIS installer, v0.2.4-to-v0.2.5
  upgrade smoke) run in release CI on a Windows runner.

## v0.2.4

- Personal cards (`.sshlaunchpad-card`) carry controller public keys, SSH
  port, display labels, network mode, and an optional Tailscale auth key from
  the controller to a new device, prefilling the wizard and starting from the
  read-only Check. Unknown card fields are ignored for forward compatibility.
- `transport.authKey` enables one-pass unattended Tailnet bootstrap; the key
  is materialized only inside Apply and redacted from plans, journals,
  reports, and exported profiles (see `docs/threat-model.md`).
- The elevation helper consumes its credential-bearing request before Apply;
  cancellation also removes it. Exact-key and wrapped-key redaction now covers
  command output, failure text, journals, and reports.
- Local candidate validation covers Go unit/race/vet and security checks,
  Windows/macOS cross-compilation, Windows PowerShell 5.1 and PowerShell 7
  Pester, frontend typecheck/build and 13 browser scenarios, Wails/NSIS, a
  v0.2.3-to-v0.2.4 installer upgrade/uninstall smoke, release packages,
  checksums, SBOM, and secret scans.

## Current product

- A beginner-first Chinese/English desktop wizard for setting up, checking, and
  repairing remote access without editing YAML or starting as administrator.
- A matching beginner CLI wizard, stable non-interactive JSON mode, and
  bilingual double-click launchers.
- A shared Go Check/Plan/Apply/Verify/Rollback engine for Windows, Linux, macOS,
  and a distinct WSL target layer.
- Public-key onboarding that distinguishes the target computer from the
  controller, never transports private keys, and keeps host-fingerprint
  verification visible.
- Tailnet-only recommended exposure, source- and port-aware firewall planning,
  self-cut protection, process locks, rollback journals, and external
  verification guidance.
- Standalone portable bundles, bootstraps, offline help, and dependency-pack
  builders. The tool itself runs offline; the Windows Tailscale adapter accepts
  a user-supplied, checksummed installer. Offline OpenSSH installation must be
  performed separately with trusted platform servicing tools.

## Validation boundary

- No SSH, Tailscale, RDP, or firewall Apply was run against the development
  workstation or any remote host. Linux and macOS system-changing behavior is
  locally validated through generated-command tests and temporary-file
  recovery fixtures, not real service changes. Native CI and isolated target
  acceptance remain release gates.
- The Windows installer is not code-signed, and macOS artifacts are not
  notarized.
- What was verified for each release is recorded in `CHANGELOG.md`.

## Release assets

- Unsigned Windows x64 GUI installer.
- Windows x64/ARM64, Linux x64/ARM64, and macOS x64/ARM64 portable CLI bundles.
- Standalone bilingual bootstrap bundle.
- SHA-256 manifest and SPDX JSON SBOM.
