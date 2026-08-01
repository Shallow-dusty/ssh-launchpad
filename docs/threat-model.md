# Threat model

## Assets

- Remote access availability.
- Authorized public keys and SSH authentication policy.
- Host firewall and service configuration.
- Control-channel continuity.
- Release binary and profile integrity.
- Local logs and journals that may reveal host details.
- Tailscale auth keys supplied for unattended bootstrap.

## Primary threats and controls

| Threat | Control |
| --- | --- |
| Malicious or replaced download | HTTPS, release manifest, mandatory SHA-256, explicit mirror/proxy; offline executables are hash-pinned and copied to privileged staging before execution |
| Secret committed to a profile | validation rejects private-key material; release scan and package smoke |
| Accidental public exposure | tailnet-only default; explicit LAN/custom CIDRs; port and scope plan |
| Lockout from restarting the only path | active transport detection, self-cut block, delayed action, external verify |
| Partial Apply leaves a broken host | pre-change validation, journaled actions with a corruption-flagging digest (mismatch warns, never blocks recovery), idempotent rollback, reversible actions, optional auto-rollback |
| Confirmed plan changes before Apply | canonical plan digest; changed profile, evidence, or commands require review and confirmation again |
| UI bypasses safety policy | UI and CLI call the same engine; executor owns confirmation gates |
| Read-only command escalates | Check/Plan/Verify never invoke elevation |
| WSL result misrepresents Windows | separate platform identity and adapters |
| Firewall uncertainty is mistaken for safety | unknown/disabled providers and extra source scopes are Plan blockers |
| Uninstall removes unrelated files | fixed install directory; uninstaller deletes only owned files and removes the directory only when empty |
| Log leaks device details | local reports use restrictive permissions; artifacts are excluded from releases |
| Tailscale auth key leaks through plans, journals, exports, or reports | the reviewable plan carries only a marker command; exact-key plus format-aware redaction covers action output, errors, journals, and reports; profile YAML exports strip the field |
| Elevation leaves a serialized auth key behind | the integrity-checked request is current-user-only, consumed and deleted by the helper before Apply, removed by the parent after cancellation/completion, and covered by stale-job cleanup after abnormal termination |
| A shared personal card leaks the optional auth key | the key is optional, one-time keys are recommended, and the UI and README instruct treating the card as a credential file sent only to devices the user intends to control |

Support-report redaction is heuristic, not a cryptographic guarantee. Review a
report before sharing it and do not distribute one that still contains a
private endpoint, account identifier, credential, or other sensitive value.

A profile `transport.authKey` opts into unattended bootstrap: Tailscale joins
the tailnet inside the same Apply that later configures SSH and the firewall,
instead of the default phased setup that pauses for an interactive sign-in.
Two accepted trade-offs: the key briefly appears in the child process argv
while `tailscale up` runs (use one-time or short-lived keys), and joining the
tailnet is intentionally irreversible by rollback — leaving a network is an
account-level decision, and joining on its own does not expose SSH.

An elevated Apply necessarily transfers the profile across the privilege
boundary. The auth key therefore exists briefly in an access-restricted,
integrity-checked request file before the helper consumes it. A hard process or
machine crash can leave that file until the next stale-job cleanup; use
one-time or short-lived keys and revoke any key whose workflow was interrupted.

## Current out of scope

- Protecting a host already controlled by an administrator-level attacker.
- Managing private keys or acting as a certificate authority.
- Replacing endpoint security, MDM, or enterprise firewall policy.
- Guaranteeing availability when no independent recovery path exists.
- Code signing and macOS notarization.

## Deliberate simplifications

Three mechanisms were reviewed as over-engineered for this threat model and
simplified after the v0.2.3 hardening (the full-hardening implementation is
preserved on the `archive/v0.2.3-full-hardening` branch). Do not re-add them
without a concrete threat:

- Effective-SSH probing uses a single global `sshd -T` dump instead of a
  per-connection `sshd -T -C` matrix; `Match`-dependent policy fails closed as
  unchecked (see `docs/platform-support.md`).
- The Windows rollback journal drops the owner-SID check; reparse-point and
  directory rejection remains, matching the unix `O_NOFOLLOW` checks.
- The interactive process lock is an exclusive-create PID file with stale-PID
  recovery instead of random tokens and compare-before-delete; the residual
  unlock race is accepted as harmless.
- The rollback journal digest is self-computed, so it only flags accidental
  corruption; a mismatch warns and rollback continues instead of refusing the
  recovery path over a checksum it could recompute.
- The GUI no longer re-plans before elevation: the reviewed plan's no-change
  and elevation flags act as routing hints, while the authoritative digest
  check stays inside the engine's own re-plan in Apply, failing closed on
  drift.
