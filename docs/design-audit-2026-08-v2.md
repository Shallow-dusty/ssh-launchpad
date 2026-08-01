# Design audit v2 — product-level review of v0.2.5

Date: 2026-08-01
Scope: every screen, every user-visible string, every interaction path of the
desktop GUI, reviewed against the product's stated audience (beginners who do
not know SSH). Triggered by the owner's field report on a real Windows
machine (v0.2.5 release build), with seven flagged problems; this audit
confirms all seven and documents the rest.

Evidence: owner screenshots (real machine, dark theme) + 15 fresh captures
under mock mode (`/tmp/audit2`, light theme, both languages, 2560/1440/390px).
The first audit (`design-audit-2026-08.md`) fixed mechanics; this one is
about product design: information architecture, semantics, copy, hierarchy.

## 0. Root cause: the UI renders the engine, not the task

The GUI is a faithful visualisation of the pipeline: probe → plan → apply →
verify. Every engine artifact leaks into the product surface:

- the probe report is shown as a raw JSON dump ("View details");
- the plan's action list is presented to the user as "还差 5 步" / "5 items";
- the wizard steps are the engine stages with thin renaming;
- engine nouns are the UI vocabulary: 方案 (plan), 信息卡 (card), 项目
  (items), 桌面核心 (desktop core), 配对文件 (pairing file), 目标系统层
  (target layer).

A beginner product must do the opposite: decide for the user, speak in
outcomes, and show only decisions that genuinely change the result. Almost
every finding below is a symptom of this single inversion. The "demo feel"
the owner reports is the flat visual hierarchy that results when every
engine artifact gets the same white card.

Severity: P0 = breaks trust/comprehension for the target user; P1 = real
friction or wrong mental model; P2 = polish.

## A. Global shell / header

- **A1 (P1) Native language `<select>`**: on Windows dark mode the opened
  dropdown renders in unthemed native light colors (owner screenshot 2). It
  also looks like a form field, not a switcher. Replace with a segmented
  `中文 / EN` toggle or a small custom menu.
- **A2 (P1) "桌面核心已连接 / Desktop core connected" pill**: meaningless to
  the audience (what is a "desktop core"?), non-actionable, permanently
  occupying header space. Show status only when something is wrong (backend
  missing → blocking warning); remove the healthy-state pill.
- **A3 (P2) Theme toggle is an unlabeled moon icon**; and there is no
  `prefers-color-scheme` default — the app always boots light unless the
  user previously toggled (the owner's machine showed dark only because of a
  stored preference).
- **A4 (P2) The brand block is a home button** with no affordance; users
  won't discover it (the wizard has its own back link, so it is redundant
  there).
- **A5 (P2) Skip-link text is hardcoded bilingual** ("跳到主要内容 / Skip to
  main content") instead of going through i18n.

## B. Home (owner shots 1/3; captures 01/02/13/14)

- **B1 (P0) Layout collapses to a narrow strip on wide windows.** Content
  max-width 960px centered: at 2560px ~63% of the window is empty margin
  (capture 01). This is the single biggest "demo look" driver. Either use a
  real app layout (wider content, multi-column where appropriate) or
  constrain the window.
- **B2 (P0) Inverted information hierarchy.** First card is a static
  explainer ("先分清两台电脑", styled like the clickable cards but not
  clickable), second is the personal-card strip (a power feature with two
  buttons and Tailscale jargon), and only the third card is the primary
  task. A beginner's eye lands on everything except the one thing they
  should do.
- **B3 (P1) Five cards, four semantics**: teaching card / feature strip with
  buttons / recommended nav card / plain nav cards. Nothing visually
  distinguishes "click me to start" from "just information".
- **B4 (P2) Doubled "recommended"**: the setup card carries both a 推荐
  badge and body text starting with "推荐路径。".
- **B5 (P1) Security footnote in pole position**: "私钥永远不会上传或写进
  配置" answers a worry the first-run user has not formed yet; it belongs in
  the key step where keys are actually handled.
- **B6 (P1) "你想做什么？" oversells choice**: there is exactly one primary
  goal. The home should be a hero action ("让这台电脑可以被远程连接" as a
  dominant button with a one-line promise) plus quiet secondary links
  (repair / card / advanced), not a wall of equivalent cards.
- **B7 (P2) Mobile role card**: the 1→2 arrow pair wraps awkwardly at 390px
  (capture 14).

## C. Check step (owner shots 4/5; captures 03/04/05)

- **C1 (P0, owner #4) The checking state is an empty panel with a spinner.**
  A real probe takes seconds; nothing communicates what is being read.
  Render the four check items as a list that lights up progressively (even a
  paced reveal beats a void).
- **C2 (P0, owner #5) "还差 5 步" is semantically wrong.** These are the
  plan's work items, not the user's steps; the phrasing reads like a
  progress penalty on a fresh machine. The check verdict should be an
  outcome sentence ("这台电脑还没有开启远程连接；需要装 1 个组件、改 4 处
  设置，全程约 2 分钟，装完自动测试") — no counts-as-steps.
- **C3 (P1) The issue list duplicates the next step.** `checkIssues()`
  derives the same five items the plan summary will show again; the user
  reads the same list twice (check screen → plan screen).
- **C4 (P0, owner #6) "查看详情" expands to a raw JSON dump** of the probe
  report (`"targetUserIsAdmin": true, …`). Unreadable for the audience.
  Either render a human fact table or remove details from the wizard and
  keep JSON in Advanced / exported reports only.
- **C5 (P2) "普通用户" fact reads like a deficiency**; the parenthetical
  (Windows will ask) carries the real information. Reframe as "安装时需要
  你点一次 Windows 确认".
- **C6 (P2) "尚未运行 / sshd"**: the service name is the note and the state
  is the value, which is fine, but "尚未运行" on a healthy fresh machine
  reads like a fault. Neutral phrasing ("未启动，安装时会启动").
- **C7 (P1) "继续" goes nowhere explicit.** After the verdict, the primary
  action should say where it leads ("下一步：确认安装内容") — or the check
  should flow straight into the plan when a plan is buildable.

## D. Plan step (owner #7; captures 06/08/12/15)

- **D1 (P0, owner #7) "确认方案" contains no decision.** Network defaults
  to recommended Tailscale, the key is auto-preselected, the plan
  auto-builds. For ~99% of runs the step is "look, then press install".
  Rename to "准备安装 / Ready to install"; collapse network and key into
  default-resolved summaries with a "更改" affordance. Show choices only
  when they matter (no Tailscale → explain; multiple/no keys → ask).
- **D2 (P1) Key area redundancy**: radio-selected detected key AND the same
  full key pasted into the textarea below it, plus four actions (import /
  generate / export pairing / paste). One key should appear once; "export
  pairing file" is a branch that belongs to the controller-side flow, not
  the middle of setup.
- **D3 (P1) Error before interaction** (capture 15): on a machine with no
  keys, the step opens with a red "还需要控制电脑的公钥" banner before the
  user has touched anything. Start neutral; escalate only after an action
  attempt.
- **D4 (P1) Network choice ignores known state**: when Tailscale is already
  connected (the owner's machine: 100.73.x), the step still presents an open
  choice instead of stating "已连接 Tailscale，将只对它开放（更改）".
- **D5 (P1) "Will open Port 22" with no why**: the default port is never
  explained or offered for change; "5 items" repeats the counting habit.
- **D6 (P1) UAC-cancel outcome is buried** (capture 12): the "No changes
  made" banner renders between the (still expanded) key picker and the plan
  summary; the user must re-read the whole page to find what happened.
  Outcomes belong at the top of the step.
- **D7 (P2) Stepper stalls on "Review the plan" during install** (capture
  08): applying is its own phase; the rail should advance to an "安装中"
  state or the stepper should reflect it.
- **D8 (P1) Install progress shows only the current event** ("1. Install …
  In progress"). There is no full checklist with pending/done states, so
  there is no sense of total progress during the slowest phase.
- **D9 (P2) Confirm dialog scrolls internally** (capture 07) when actions
  exceed its height, and the acknowledgement copy is legalistic. The dialog
  should list every action without scroll and ask a human question.

## E. Finish / verify step (capture 09)

- **E1 (P1) Two stacked green "Ready" messages** (banner + connection
  visual) say the same thing.
- **E2 (P1) The connection card's two-column layout is scattered**: heading
  floats top-right, visual block left, facts bottom-left, copy box mid-right;
  nothing leads the eye to the one action that matters (copy the command).
- **E3 (P2) "Check again" after success** is nonsensical as a primary-row
  action (re-verify is an edge case, not a peer of "Finish").
- **E4 (P0) The moment of success has no payload hierarchy.** The copyable
  command is the entire point of the product; it should be the dominant
  element, with numbered next actions (1. 复制命令 2. 到控制电脑粘贴
  3. 核对指纹) instead of a dense aside.
- **E5 (P2) No after-care**: nothing about how to change settings later,
  disconnect, or uninstall cleanly.

## F. Repair mode (capture 10)

- **F1 (P1) Contradictory CTA**: on a healthy machine the verdict says "无需
  重复改动" while the primary button still says "继续修复". It should become
  "验证连接" and route straight to verify.
- **F2 (P2) Different step vocabulary** (诊断/修复/验证 vs 检查电脑/确认方案/
  完成连接) for what is mostly the same pipeline; the rename is fine but the
  two flows should share verdict/CTA logic.
- **F3 (P2) `repairIntro` promises "只显示需要修复的项目" then shows none.**

## G. Advanced mode (capture 11)

- **G1 (P1) The personal-card editor is the first and largest panel** — a
  beginner-distribution concept headlining the power page. Move it out
  (its own dialog or a home-level affordance) or demote it below system
  settings.
- **G2 (P1) "创建信息卡" on Home jumps into "Advanced mode"** and focuses a
  required "卡名称" field — the most beginner-facing card flow is framed by
  the scariest page title.
- **G3 (P2) Flat grouping**: profile toolbar (4 buttons), 6 selects, keys
  textarea, 2 checkboxes, recovery panel, update check, unsigned notice —
  one long page without section landmarks ("配置 / 操作 / 恢复").
- **G4 (P2) Wrong toast**: exporting a report with no report says "没有改
  动" (`noChanges`) instead of "还没有检查报告".
- **G5 (P2) "这里保留 CLI 同等能力"** assumes the reader knows what a CLI
  is; the audience does not.

## H. Personal card (feature-level)

- **H1 (P1) Naming**: "个人信息卡" communicates nothing; it is a
  provisioning bundle (controller pubkeys + port + optional one-time auth
  key) for setting up *additional* computers. It should be named for its
  job ("批量装机卡 / 新电脑导入卡") and framed as an optional, second-device
  feature — not the second card on Home.
- **H2 (P2) Required "卡名称"** is a formality the user must invent on the
  spot; prefill a default ("我的远程控制") and let it be edited.
- **H3 (P2) Import jumps straight into the wizard** without confirming the
  card's contents with the user (port, keys, network) — a summary confirm
  step is warranted for a file that drives system changes.

## I. Copy audit (i18n sweep)

- **I1 (P0) "还差 {count} 步" is reused for two different semantics**
  (check verdict `missingSteps`, verify verdict `testNeeds`). Both should be
  outcome sentences; the shared counting habit must go.
- **I2 (P1) `testNeedsBody` argues with the user** ("…没有把它误报成连接成
  功") — that sentence defends the implementation, it does not help the
  reader.
- **I3 (P1) System-perspective vocabulary** throughout: 方案 / 项目 / 信息卡
  / 桌面核心 / 配对文件 / 目标系统层 / 允许来源 / 下载来源. Each needs a
  user-perspective replacement or a context where it is explained once.
- **I4 (P2) "安全网络" is a coined term** for Tailscale that appears before
  any explanation.
- **I5 (P2) `installFailedBody` / `errorPlanChanged` / 30-minute timeout
  copy** mention rollbacks, digests, and Task Manager — support content, not
  beginner guidance. Failures should say what happened, what was not
  changed, and the one next action.
- **I6 (P2) `unsignedNotice` is pinned permanently in Advanced**; it belongs
  to download/update moments.

## J. Visual system

- **J1 (P0) Everything is the same card.** Identical white surfaces, radius,
  and shadow for teaching text, status facts, choices, and primary tasks →
  zero hierarchy → the "wall of cards" demo feel. Primary actions, status
  facts, choices, and reference info need distinct visual languages.
- **J2 (P1) The stepper is three heavyweight boxes** (full-width cells,
  badge + label, ~90px tall). A slim progress indicator (bar or compact
  numbered labels) carries the same information at a third of the height.
- **J3 (P2) Iconography is uniform** (same accent tint everywhere) and does
  not encode meaning.
- **J4 (P2) `info` banners share the accent blue**, so a UAC cancellation
  and a recommendation wear the same color.
- **J5 (P2) Text density**: small-note/muted paragraphs everywhere; several
  screens read as essays.
- **J6 (P2) Dead CSS**: `.stepper` still carries the old `repeat(4, …)` rule
  overridden two lines later by `repeat(3, …)`.

## K. Interaction logic (residual)

- **K1 (P1) Pasted keys only rebuild the plan on blur** (`change`); after
  pasting, the plan below silently stays stale until the user clicks away.
  Rebuild on paste/input settle (debounced).
- **K2 (P2) Two progress-driven render paths coexist**: the Wails event
  listener re-renders on every engine event, and the poll loop re-renders on
  fingerprint change. One owner should drive rendering.
- **K3 (P2) Language switch rebuilds the entire shell** mid-wizard; state
  survives but focus and scroll do not.
- **K4 (P2) The 30-minute poll timeout tells beginners to open Task
  Manager.**

## What is genuinely good (do not regress)

- Task-worded wizard steps and the repair-mode split (v1 audit).
- Network choice cards state consequences in plain words.
- The failure semantics: retryable plan errors, persistent check errors, no
  stale-report backfill, exit-code-driven messages, self-cut block, UAC
  cancel vs failure distinction.
- Live-applied Advanced settings with no hidden saves.
- The confirm-before-elevate gate with an explicit acknowledgement.

## Redesign proposal (fix direction)

**P0 — semantics & copy (largest trust gain, mostly views.ts + i18n.ts):**
outcome-sentence verdicts (kill every "还差 N 步"/"N items"); human fact
tables instead of JSON in the wizard (JSON moves to Advanced/export); rename
the plan step to "准备安装" with default-resolved choices behind "更改";
home becomes one hero action + quiet secondary links; finish page centers on
the copyable command with numbered next steps; error copy rewritten from the
user's perspective.

**P1 — layout & hierarchy:** wider/fluid content layout (or constrained
window); distinct visual languages for action vs status vs info; slim
stepper; custom language toggle; header without the healthy-state pill;
progress checklist for apply; outcome banners anchored to the top of their
step; repair mode routes to verify when healthy.

**P2 — interaction residue:** plan rebuild on paste; single render driver;
system-theme default; i18n skip link; dead CSS; toast/copy fixes (G4, I2,
K4); personal card renamed and repositioned.

Sequencing options are discussed with the owner; P0 alone removes most of
the "demo feel" because the feeling comes from words and hierarchy more than
from pixels.
