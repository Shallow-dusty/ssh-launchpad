# Architecture

## One engine, three surfaces

```text
profile.yaml
    |
    v
probe -> snapshot -> planner -> immutable action plan
                                  |
                    +-------------+-------------+
                    |                           |
                executor                    report
          journal / rollback / events       JSON v1
                    |
       +------------+------------+
       |                         |
      CLI                    Wails Studio

standalone bootstrap scripts -> verified release artifact -> CLI or Studio
```

`internal/launchpad` owns profiles, probing, planning, execution, download
verification, journals, events, and reports. `cmd/ssh-launchpad` and `app.go`
only translate user input into engine calls. The UI does not assemble shell
commands or decide safety policy. The desktop shell passes the reviewed plan's
no-change and elevation flags as execution-path hints; the authoritative plan
digest check always happens inside the engine's own re-plan in Apply, so a
stale hint fails closed.

Secrets follow the same immutability rule. When a profile carries a
`transport.authKey`, the planner emits a marker command instead of the real
argv, so the reviewable plan and the journal never contain the key; the
executor materializes the marker into the real `tailscale up` command only
inside Apply. The plan digest still binds the key because the digest covers
the whole profile. When Windows UAC or CLI elevation is required, the
credential crosses the privilege boundary in the integrity-checked,
current-user-only request file. The helper consumes and deletes that file
before Apply; cancellation and normal parent cleanup also remove it.

`internal/elevation` owns the single versioned GUI/CLI privilege boundary.
The standard-user process writes and hashes an immutable request and
pre-creates fixed response/event files. The elevated helper verifies the
digest and same-directory paths, consumes the request, refuses reparse-point
output targets, and writes only those existing files.

## Platform boundaries

Windows uses PowerShell and Windows service/firewall providers. Linux uses
package-manager detection, systemd, sshd drop-ins, and firewalld/ufw adapters.
macOS uses system SSH, `launchd`, and platform configuration checks. A WSL
instance is a Linux target with an explicit WSL platform identity; Windows
service and firewall state is never inferred from it.

The planner produces inspectable commands, risk, elevation, reversibility, and
self-cut metadata. A canonical SHA-256 digest binds confirmation to the reviewed
profile and executable plan; Apply aborts if a fresh Plan differs. The executor
refuses unconfirmed work and writes a journaled record of intended and
completed actions before mutations. The journal carries a self-computed digest
that flags accidental corruption at rollback time; a mismatch downgrades to a
warning rather than blocking recovery. Platform commands are intentionally declarative so unit tests can
validate them on any CI runner. Windows-generated commands are
parsed by PowerShell; Unix-generated commands are checked by `sh -n` and
ShellCheck on native CI.

## Report and exit contract

Every stage returns JSON schema version 1. Exit codes are stable:

| Code | Meaning |
| ---: | --- |
| 0 | success |
| 2 | invalid profile |
| 3 | verification failed |
| 4 | elevation required |
| 5 | confirmation required |
| 6 | self-cut blocked |
| 7 | partial failure |
| 8 | download failure |
| 9 | unsupported platform or capability |

Reports keep network reachability, service health, effective authentication,
configuration validity, authorized-key evidence, and firewall confidence as
separate fields. Verify fails closed when authentication or firewall evidence is
unknown.
