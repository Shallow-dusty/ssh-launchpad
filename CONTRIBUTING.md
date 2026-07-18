# Contributing

Open an issue before adding a new privilege boundary, package source, firewall
provider, or control-channel behavior.

Changes should:

- preserve the Check/Plan read-only and Verify non-elevating contracts;
- keep generic behavior in `internal/launchpad`;
- add tests for generated commands and failure behavior;
- avoid real host data in fixtures;
- document platform-specific verification limits.

Run the checks in `docs/release-verification.md`. Commit messages and public
discussion should describe the problem, implementation, and objective test
evidence.
