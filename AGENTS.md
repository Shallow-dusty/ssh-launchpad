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
  Keep raw reports/journals in ignored output directories, never in tracked
  source or the repository root. Preserve old acceptance evidence.

## Documentation and layout

- `STATUS.md` owns the current version, validation baseline and open limits;
  `ROADMAP.md` owns pending work; `CHANGELOG.md` owns completed version history.
- Current topic guides stay in `docs/`; dated audit/design/acceptance summaries
  go in `docs/records/`. Superseded material goes in `docs/90.Archive/`, with
  an `ARCHIVE_NOTE.md` stating source, reason and active replacement.
- Update `docs/README.md` and inbound references when moving documents.
  Document cleanup never renews test evidence or closes an acceptance gate.
- Root Go bridge files, `build/appicon.png`, and `frontend/dist` have existing
  Wails build roles. Consult `docs/development.md` before relocating them.
