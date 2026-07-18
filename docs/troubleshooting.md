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

Exit 4 means elevation is required. Exit 5 means the high-risk confirmation was
not supplied. Exit 6 means an action could interrupt the active control channel.
Review the JSON plan rather than bypassing the gate.

For a self-cut plan, establish a second control path first. Then use a delayed
action with a controller-visible `host:port` external verification target. Do
not restart Tailscale or SSH from the only session carried by that component.

## Partial failure

The report contains `journalPath` and per-action results. If auto-rollback did
not finish, run:

```text
ssh-launchpad rollback --journal <journal.json> --output rollback.json
```

Inspect the rollback report from a local console or an independent channel.
Rollback can only restore actions marked reversible; package installation and
external policy may require manual repair.

## Download failure

- Confirm the release tag and asset name.
- Confirm `checksums.txt` is next to an offline asset.
- Treat a hash mismatch as a security or cache-corruption event.
- Use an explicit HTTPS mirror or proxy; never disable certificate checking.
- Preserve the failed `.part` file if diagnosing resume behavior.
