# Repository working agreement

Personal project: a beginner-first Chinese/English wizard that sets up SSH
remote access (GUI + CLI). It modifies sshd, firewall rules, and
authorized_keys on real machines, so the rules below are product safety
contracts, not process theater.

## Safety contracts

- Never commit private keys, tokens, real device profiles, exported logs, or
  host identities.
- `check` and `plan` are read-only; `verify` never requests elevation.
- Never weaken TLS verification or execute downloaded script text; downloads
  require HTTPS + SHA-256.
- A change that could cut the active SSH/Tailscale path is blocked by default
  and needs a rollback journal plus an external verification path.

## Development

- Read `STATUS.md` and the relevant `docs/` file before editing.
- Keep platform commands behind the planner/executor interfaces; the UI never
  assembles shell commands or decides safety policy.
- When changing planner output, Apply, rollback, or download verification, add
  or update the matching tests.
- Generated files go under `build/`, `dist/`, or `frontend/test-results/`.
  Don't leave browser captures, reports, or local journals in the repo.
