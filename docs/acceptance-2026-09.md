# Real-machine acceptance, 2026-09-15/16 (v0.2.6 candidate)

This is the first acceptance round that mutates real SSH, firewall and
transport state on a disposable host. Earlier rounds were local-only
(unit/race tests, generated-command syntax checks, browser tests). Every
finding below produced a code change, a regression test, or an explicitly
documented boundary.

## Environment

| Item | Value |
| --- | --- |
| Host | Azure VM `slp-accept`, Windows Server 2022 Datacenter (10.0.20348), Standard_B2s, Trusted Launch |
| Initial state | no `sshd`, no OpenSSH capability, no Tailscale, no matching firewall rules, **no winget** |
| Account | built-in Administrator (renamed `slpadmin`) with auto-logon, so Apply/Verify/installer ran in a real user session |
| Delivery | Azure Run Command + a private blob container; every artifact verified against `SHA256SUMS` before execution |
| Transport | Tailscale node `slp-accept` (`tag:debug`) joined with a one-time auth key; controller reached it from the maintainer's host over the tailnet |
| Cleanup | resource group, storage account, tailnet device, auth keys, and local secrets deleted after the run (see *Cleanup*) |

Server 2022 differs from Windows 10/11 clients (no winget, no consumer
software, SYSTEM-context Run Command). The findings below are therefore
reported with their observed scope, not as universal claims.

## Verified end to end

1. **OpenSSH capability install** (`download.strategy: official`): the plan
   contained only the phased `install_ssh` action; after the install a fresh
   Check/Plan produced `configure_sshd`, `configure_authorized_keys`,
   `configure_firewall`, `enable_sshd`, and Apply completed 4/4 with a journal.
2. **Verify and invariants**: password authentication disabled, public-key
   authentication enabled, `sshd` Running with Automatic start, the stock
   `OpenSSH-Server-In-TCP` rule disabled, and the managed rule
   `SSH-Launchpad-TCP-22` scoped to the requested `10.0.0.0/24`. A second
   Apply was a no-op (idempotence confirmed).
3. **Tailscale join** (pinned offline bundle, SHA-256 verified) plus tailnet
   exposure: `install-tailscale`, `authenticate-tailscale`,
   `configure-firewall` all applied; **a real SSH login over the tailnet
   succeeded with public-key authentication only and the password path was
   refused** (host key fingerprint `SHA256:IKXMONSR7ULkedOy+ATX0gTvRppcwFtd2TrazHb8jro`).
4. **Port change (22 → 2222) driven from inside the SSH session**: all three
   self-cut refusals behaved as designed — plain Apply exited 6, `--allow-self-cut`
   without an external verification target exited 6, an unreachable external
   target exited 6 — with no side effects. After the run through Run Command,
   SSH answered on 2222, timed out on 22, and still refused passwords.
5. **Rollbacks**: the port-change journal restored port 22 and SSH
   reachability; the tailnet journal restored the `10.0.0.0/24` scope; the
   capability journal stopped and disabled `sshd` again.
6. **Installer lifecycle** (per-user NSIS, run as a plain user): released
   v0.2.5 installed, the v0.2.6 candidate upgraded it in place (registry
   `DisplayVersion` 0.2.5 → 0.2.6, executable product version 0.2.6, install
   marker `SSH Launchpad/0.2.6`), and the uninstaller removed the application,
   shortcuts and registry entry. With the install marker deleted the
   uninstaller correctly refused to remove application files.

## Defects found on the real host and fixed

| # | Symptom | Fix |
| --- | --- | --- |
| 1 | A freshly provisioned Windows host has no `sshd_config`; the probe treated that documented intermediate state as a hard failure, dead-ending the phased flow | Windows-only sentinel (`errSSHPolicyNotInitialized`) plus `Snapshot.SSHPolicyNotInitialized` and a plan warning; Unix keeps failing closed |
| 2 | The config action could not write `C:\ProgramData\ssh` because the directory does not exist yet | Create the directory before writing the managed block |
| 3 | `sshd -t` rejected the new config ("no hostkeys available") on a host that had never started sshd | Generate host keys (`ssh-keygen -A`) with SYSTEM/Administrators-only ACLs **and** SYSTEM ownership, in both the config and enable actions |
| 4 | Re-scoping failed because the tool treated its own previous rule scope as foreign exposure | `FirewallState.ManagedScopes` and rule ownership, so the managed rule's own scope is a reconfiguration |
| 5 | A successful rollback left `.bak`/`.created`/`.ready` markers behind, so retrying Apply reported "backup already exists" forever | Rollback commands consume their markers on success (Windows config/keys, Unix config/keys) |
| 6 | Tailscale's own `Tailscale-In` rule (all ports, any source) blocked every plan | `FirewallState.ThirdPartyBroadRules` recognises third-party broad rules and warns precisely instead of blocking |
| 7 | After a port change the previous managed rule stayed enabled and was reported as conflicting exposure | `FirewallState.StaleManagedRules`; the planner disables stale managed rules with recorded rollback state |
| 8 | Rollback reported failure when another rollback had already removed the managed rule | Recovery became idempotent: an absent rule is reported, not an error |
| 9 | **Windows rollback commands ending in `Remove-Item … -ErrorAction SilentlyContinue` exited 1** when the file no longer existed, so a completed rollback was reported as `rollback-failed` and the remaining recovery actions never ran | Guarded cleanup (`if(Test-Path …){ Remove-Item -ErrorAction Stop }`) in the config, keys and firewall rollbacks, with `TestWindowsRollbacksDoNotFailOnAbsentBackups`; a live journal that previously ended in `rollback-failed` now returns exit 0 and removes every marker |
| 10 | The generated PowerShell syntax test planned only the phased install action, so config/keys/firewall commands were never parsed — a missing brace in the new firewall rollback reached the real host undetected | Fixture now plans the full action set and `TestSyntaxFixtureCoversWindowsFirewallAction` asserts that coverage |

Defect 9 is the most consequential: a false rollback failure is indistinguishable
from a real one and aborts the recovery chain, so an operator could believe a
host was restored when only part of it was.

## Boundaries and open items

- **Rollback is fail-fast.** A genuinely failed rollback action stops the
  remaining recovery actions; the journal stays `rollback-failed` and the GUI
  surfaces it for review. Observed live on the first stage-A rollback (a
  firewall rollback failure stopped key and config recovery). Defect 9 removes
  the false-failure trigger, but the policy itself is unchanged and intentional.
- **Scope guarantees are limited by other products.** While the Tailscale
  `Tailscale-In` rule exists, SSH stays reachable from the tailnet even when the
  tool's own rule is scoped back to a private CIDR. The plan warns; the tool
  cannot enforce what another product's rule admits.
- **Silent uninstall leaves `uninstall.exe` and the install directory** behind
  (the NSIS uninstaller cannot delete itself while running). Application files,
  shortcuts and registry entries are removed.
- **Marker-refusal is now diagnosable.** Deleting `.ssh-launchpad-install`
  makes the uninstaller refuse to delete application files; it now also sets
  exit code 68, so a silent, scripted uninstall can tell a refusal from a
  completed removal (observed exit 0 during this round, fixed afterwards and
  guarded by a release-metadata test).
- **Installer layout**: per-user install in
  `%LOCALAPPDATA%\Programs\SSH Launchpad`, uninstall entry under
  `HKCU\…\Uninstall\SSH Launchpad ContributorsSSH Launchpad` (the Wails
  template concatenates company and product name). Tools and docs should use
  that key, not `…\Uninstall\SSH Launchpad`.
- **No winget on Server 2022** makes a single global `download.strategy`
  insufficient: `official` blocks the Tailscale step, `offline` blocks the
  OpenSSH capability step. Acceptance therefore ran in two stages. This is a
  real product limitation for winget-less hosts, not a test artifact.
- **Not covered**: Windows 10/11 client behaviour, GUI/UAC click-through
  (Run Command runs as SYSTEM), WebView2-less hosts, macOS/Linux real hosts,
  and the UFW raw-rule gap already documented in `docs/audit-2026-09.md`.
- **The unelevated Apply gate (exit 4 `NeedsElevation`, no changes) was not
  observable on this host.** The built-in Administrator account runs with UAC
  token filtering off, so its "unelevated" processes still hold a full token;
  Run Command itself runs as SYSTEM. A `runas /trustlevel:0x20000` attempt
  produced no usable signal. Windows Sandbox is unavailable on Windows 11 Home,
  so the next round should either create a real standard user (and apply the
  plan from that session) or set `FilterAdministratorToken=1` before testing.
  The gate's decision logic is unit-tested locally; only the end-to-end
  behaviour on Windows remains unverified.

## Next acceptance round

Ordered by risk, for a disposable host (Azure again is cheapest; Windows
Sandbox is not available on Windows 11 Home):

1. Unelevated Apply gate: standard user (or `FilterAdministratorToken=1`) →
   expect exit 4 with no configuration, firewall, or service change.
2. Windows 10/11 client host with winget present: official strategy for both
   OpenSSH and Tailscale in one pass, plus the WebView2-less case.
3. Re-run the installer round: marker refusal must exit 68, and confirm the
   silent-uninstall leftovers (`uninstall.exe` plus the install directory).
4. Linux (systemd + UFW, then firewalld) and macOS hosts for the Unix paths,
   including the documented UFW raw-rule gap.
5. Optional: long-running interruption tests (power loss / kill during Apply)
   to exercise write-ahead recovery outside the unit fixtures.

## Cleanup

The acceptance environment was destroyed afterwards: resource group
`rg-slp-accept-20260915` (VM, OS disk, NIC, public IP, VNet, NSG, storage
account) deleted, the tailnet device `slp-accept` removed, the leftover
`slptest` device from the 2026-09-04 round removed, the minted auth keys
verified absent, and local secrets (VM password, container SAS, auth key file)
deleted. No tailnet ACL changes were made.

## Evidence

Raw harness output stays local under `build/audit/azure-20260915/` (gitignored):
`plan.result.json`, `apply-user.result.json`, `verify.result.json`,
`invariants.result.json`, `rollback.result.json`, `installer.result.json`,
`rc-*.json`, `journals.result.json`, plus the `dist/SHA256SUMS` used for the
artifact uploads. The harness itself (`accept.ps1`, `installer-task.ps1`,
`rc.sh`) is reproducible from that directory.
