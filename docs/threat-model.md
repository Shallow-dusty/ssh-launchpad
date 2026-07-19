# Threat model

## Assets

- Remote access availability.
- Authorized public keys and SSH authentication policy.
- Host firewall and service configuration.
- Control-channel continuity.
- Release binary and profile integrity.
- Local logs and journals that may reveal host details.

## Primary threats and controls

| Threat | Control |
| --- | --- |
| Malicious or replaced download | HTTPS, release manifest, mandatory SHA-256, explicit mirror/proxy |
| Secret committed to a profile | validation rejects private-key material; release scan and package smoke |
| Accidental public exposure | tailnet-only default; explicit LAN/custom CIDRs; port and scope plan |
| Lockout from restarting the only path | active transport detection, self-cut block, delayed action, external verify |
| Partial Apply leaves a broken host | pre-change validation, journal, reversible actions, optional auto-rollback |
| UI bypasses safety policy | UI and CLI call the same engine; executor owns confirmation gates |
| Read-only command escalates | Check/Plan/Verify never invoke elevation |
| WSL result misrepresents Windows | separate platform identity and adapters |
| Log leaks device details | local reports use restrictive permissions; artifacts are excluded from releases |

## Out of scope for v0.2.0

- Protecting a host already controlled by an administrator-level attacker.
- Managing private keys or acting as a certificate authority.
- Replacing endpoint security, MDM, or enterprise firewall policy.
- Guaranteeing availability when no independent recovery path exists.
- Code signing and macOS notarization.
