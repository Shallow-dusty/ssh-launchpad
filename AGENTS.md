# Repository working agreement

SSH Launchpad is the single source for reusable SSH bootstrap, planning, apply,
verification, and recovery logic.

## Boundaries

- Never commit private keys, authentication tokens, real device profiles,
  exported logs, or host identities.
- `check` and `plan` are read-only. `verify` must never request elevation.
- Do not weaken TLS verification or execute downloaded script text.
- A change that may interrupt the active SSH or Tailscale path must be blocked
  by default and must have a rollback journal plus an external verification
  path before it can be scheduled.
- Platform-specific repositories may consume this project and keep private
  profiles or evidence, but must not copy the generic implementation.

## Development flow

1. Read `README.md`, `STATUS.md`, and the relevant document under `docs/`.
2. Run `git status --short --branch` before editing.
3. Keep platform commands behind planner and executor interfaces.
4. Add tests for planner output, repeat Apply, partial failure, rollback, and
   download verification when changing those behaviors.
5. Before release, run the checks in `docs/release-verification.md`, inspect the
   artifact contents, scan for secrets and device identity, and update
   `CHANGELOG.md`.

Generated files belong under `build/`, `dist/`, or `frontend/test-results/`.
Do not leave browser captures, reports, or local journals in the repository.
