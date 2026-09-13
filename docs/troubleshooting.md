# Troubleshooting and recovery

## Read the layers in order

1. `check`: is the client/server installed and which target is being inspected?
2. `plan`: which exact service, configuration, key, firewall, or transport
   action differs?
3. `verify`: is config syntax valid, is the service running, is the expected
   port listening, is its firewall scope correct, and can the protocol be
   reached?
4. From a separate controller, test KEX and then authentication. Do not infer
   key acceptance from TCP reachability.

## Apply stopped before changing anything

Exit 4 means elevation is required. Exit 5 means confirmation or the reviewed
`plan.digest` is missing/stale, or another mutation holds the system lock; inspect
the error, wait for the active run, then run Plan again and review it. Exit 6 means an
action could interrupt the active control channel. Review the JSON plan rather
than bypassing the gate.

For a self-cut plan, establish a second control path first. Then use a delayed
action with a controller-visible `host:port` external verification target. Do
not restart Tailscale or SSH from the only session carried by that component.
`--schedule-risky` retains the lock during an in-process cancellable delay;
keep the process running. Even an explicit risk override needs an external
endpoint. TCP reachability alone does not establish an independent channel.

## Partial failure

The report contains `journalPath` and per-action results. If auto-rollback did
not finish, run:

```text
ssh-launchpad rollback --journal <journal.json> --output rollback.json
```

Inspect the rollback report from a local console or an independent channel.
Rollback can only restore actions marked reversible; package installation and
external policy may require manual repair. GUI failed-Apply pages also offer
Recover reversible changes and redacted-report export. A failed recovery is not
permission to repeat the previous Apply without a fresh Check/Plan.

Legacy journals containing detached scheduled work need manual verification of
that task before restoring files. Interrupted irreversible actions remain
incomplete recovery. Do not remove journals to suppress these warnings.

## Supported-policy blockers

- Custom SSH Match/ListenAddress, included Port or recursive Include: inspect
  existing policy manually instead of deleting it just to satisfy the wizard.
- UFW: the adapter reads `status verbose`; custom before/after raw rules or
  independent nftables/iptables rules are not fully inventoried. Review these
  externally; no complete firewall safety claim is made for such hosts.
- firewalld: multiple zones, unsupported services/policies/direct rules and
  runtime/persistent drift need explicit operator review.
- Missing OpenSSH repair is phased: complete installation, then run a fresh
  Check/Plan. An incomplete Verify offers Review remaining changes.
- A long-running helper is not cancelled by a frontend status deadline. Keep
  tracking its terminal report and do not start another installation.

## Download failure

- Confirm the release tag and asset name.
- Confirm `checksums.txt` is next to an offline asset.
- Treat a hash mismatch as a security or cache-corruption event.
- Use an explicit HTTPS mirror or proxy; never disable certificate checking.
- Preserve the failed `.part` file if diagnosing resume behavior.
