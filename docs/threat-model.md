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
| Partial Apply leaves a broken host | pre-change validation, integrity-digested journal, idempotent rollback, reversible actions, optional auto-rollback |
| Confirmed plan changes before Apply | canonical plan digest; changed profile, evidence, or commands require review and confirmation again |
| UI bypasses safety policy | UI and CLI call the same engine; executor owns confirmation gates |
| Read-only command escalates | Check/Plan/Verify never invoke elevation |
| WSL result misrepresents Windows | separate platform identity and adapters |
| Firewall uncertainty is mistaken for safety | unknown/disabled providers and extra source scopes are Plan blockers |
| Uninstall removes unrelated files | fixed install directory; uninstaller deletes only owned files and removes the directory only when empty |
| Log leaks device details | local reports use restrictive permissions; artifacts are excluded from releases |
| Tailscale auth key leaks through plans, journals, exports, or reports | the reviewable plan carries only a marker command and the key is materialized inside Apply; action output and support reports redact `tskey-auth-` values; profile YAML exports strip the field |

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

## Current out of scope

- Protecting a host already controlled by an administrator-level attacker.
- Managing private keys or acting as a certificate authority.
- Replacing endpoint security, MDM, or enterprise firewall policy.
- Guaranteeing availability when no independent recovery path exists.
- Code signing and macOS notarization.
