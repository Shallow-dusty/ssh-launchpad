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
commands or decide safety policy.

## Platform boundaries

Windows uses PowerShell and Windows service/firewall providers. Linux uses
package-manager detection, systemd, sshd drop-ins, and firewalld/ufw adapters.
macOS uses system SSH, `launchd`, and platform configuration checks. A WSL
instance is a Linux target with an explicit WSL platform identity; Windows
service and firewall state is never inferred from it.

The planner produces inspectable commands, risk, elevation, reversibility, and
self-cut metadata. The executor refuses unconfirmed high-risk work and writes a
journal before mutations. Platform commands are intentionally declarative so
unit tests can validate them on any CI runner.

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

Reports keep network reachability, service health, configuration validity, and
authentication as separate evidence.
