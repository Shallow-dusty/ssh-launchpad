# UI refinement — 2026-09-13

## Scope and reference decisions

Keep the Wails + vanilla TypeScript/CSS three-step wizard, existing components,
i18n keys, and planner/executor separation. The UI never constructs mutation
commands or overrides safety blockers. Remote-Onboarder remains a separate
single-EXE product; it is not merged into the wizard.

Useful lessons from the working OneClick flow: a recipient should make fewer
choices, see explicit phase feedback, and finish with a shareable connection
summary. Do not borrow its old unconditional retry/success promises or force
reauthentication behavior.

Local Gallery retrieval: `theme=desktop`, `style=matte-elevation`,
`my_tags=candidate`; read Liquid Glass entry/overview and the user-provided
Matte Elevation reference (`05-matte-elevation.png`). Observed facts: opaque
surfaces, restrained edges, soft shadows, clear foreground/background depth.
Borrow those structural choices, not refraction, chromatic glow, or decorative
percentages. The Gallery entry remains a candidate; no new reference was
promoted and no universal aesthetic ranking was inferred.

## Changes

- Home: three concrete steps, target/controller roles, read-only-first promise,
  and no arbitrary two-minute estimate or blanket reversibility guarantee.
- Wizard: persistent three-step location, keyboard focus across navigation and
  asynchronous rerenders, and explicit irreversible-action labels.
- Probe waiting: neutral pending indicators, not timed fake green completion.
- Failed Apply: journal-based recovery and redacted-report actions in context,
  not hidden only in advanced settings. Busy recovery rerenders controls;
  retry requires fresh review rather than resubmitting a stale failed plan.
- Verification: show remaining actions/blockers and offer fresh review rather
  than an apparent Finish action. This supports phased dependency repair.
- Handoff: copy connection details without keys; clearly separate local checks
  from a real controller-to-target connection.
- Styling: reuse panel/summary/button/banner tokens, improve dark-button
  contrast, balance headings, add restrained flow cards, preserve small-screen
  layout and reduced-motion behavior. Existing icons are reused.

## Validation

Production build and typecheck passed. Playwright: **23 tests passed**, including
four additional flow/visual checks. Existing bridge tests cover retained failed
journals, immediate job completion and helper timeouts. New captures cover home
light/dark, 360px dark home, review, pending Verify, and successful handoff.
A failed-Apply recovery-page capture is also included. Screenshots are in `frontend/test-results/` and were visually inspected; not
committed. Verified keyboard focus, copy content, overflow, current-step ARIA,
and existing keyboard/skip-link tests. This is not a screen-reader certification
or Windows WebView2 pixel parity claim.

The test server serves `frontend/dist`, so `pnpm run test:e2e` now automatically
builds first to avoid verifying a stale production bundle. No live system mutation, subagent,
real UAC operation, installer run, commit, or release was performed.
