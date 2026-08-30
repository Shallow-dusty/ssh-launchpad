import type { Language, MessageKey } from "./i18n";
import { escapeHtml } from "./browser-utils";
import {
  arrowIcon, backIcon, checkIcon, closeIcon, copyIcon, downloadIcon, infoIcon,
  keyIcon, launchIcon, lockIcon, monitorCheckIcon, networkIcon, packageIcon,
  powerIcon, repairIcon, searchIcon, shieldIcon, slidersIcon, uploadIcon,
  warningIcon
} from "./icons";
import type { ElevatedJob, PlanAction, Profile, PublicKeyInfo, Report, Snapshot } from "./types";

export type WizardMode = "setup" | "repair";
export type InstallState = "idle" | "waiting-for-permission" | "running" | "failed" | "cancelled" | "completed";
type Translate = (key: MessageKey, values?: Record<string, string | number>) => string;

export interface ViewState {
  language: Language;
  mode: WizardMode;
  step: number;
  profile: Profile;
  personalCard: {
    displayName: string;
    controllerName: string;
    note: string;
  };
  report?: Report;
  planReport?: Report;
  planError: string;
  verifyReport?: Report;
  busy: boolean;
  detectedKeys: PublicKeyInfo[];
  selectedKey?: PublicKeyInfo;
  progress: Array<{ kind: string; message: string; actionId?: string }>;
  installState: InstallState;
  installError: string;
  checkError: string;
  verifyError: string;
  activeJob?: ElevatedJob;
  // Prepare-step expanders ("更改" affordances; choices stay collapsed otherwise).
  showNetwork: boolean;
  showKey: boolean;
  // Set once the user attempted to continue without a key, so the missing-key
  // panel stays neutral until an actual attempt (no error-on-arrival).
  keyAttempted: boolean;
}

// ---------------------------------------------------------------- home

export function renderHome(state: ViewState, t: Translate): string {
  return `
    <div class="home">
      <section class="hero">
        <p class="eyebrow">${t("version")}</p>
        <h1>${t("homeHeroTitle")}</h1>
        <p class="lead">${t("homeHeroLead")}</p>
        <button id="hero-start" class="button primary hero-action">${launchIcon()} ${t("homeHeroAction")}</button>
        <p class="hero-meta">${t("homeHeroMeta")}</p>
      </section>
      <p class="roles-note">${infoIcon()}<span>${t("homeRolesNote")}</span></p>
      <nav class="home-links" aria-label="${t("wizardSetup")}">
        <button id="repair-link" class="text-link">${repairIcon()} ${t("homeRepairLink")}</button>
        <button id="card-import-link" class="text-link">${uploadIcon()} ${t("homeCardLink")}</button>
        <button id="advanced-link" class="text-link">${slidersIcon()} ${t("homeAdvancedLink")}</button>
      </nav>
      <footer class="privacy-foot"><span>${lockIcon()}</span>${t("noTelemetry")}</footer>
    </div>`;
}

// ---------------------------------------------------------------- wizard shell

export function renderWizard(state: ViewState, t: Translate): string {
  const steps = wizardSteps(state, t);
  const current = Math.min(state.step + 1, steps.length);
  return `
    <div class="wizard-header">
      <button class="text-button" id="wizard-back">${backIcon()} ${t("backHome")}</button>
      <div>
        <p class="eyebrow">${state.mode === "setup" ? t("wizardSetup") : t("wizardRepair")}</p>
        <h1 class="wizard-title">${steps[state.step] ?? steps[0]}</h1>
      </div>
    </div>
    <div class="progress-head" role="progressbar" aria-valuemin="1" aria-valuemax="${steps.length}" aria-valuenow="${current}" aria-valuetext="${t("stepOf", { n: current, total: steps.length })}">
      <span class="progress-label">${t("stepOf", { n: current, total: steps.length })} · <b>${steps[state.step] ?? steps[0]}</b></span>
      <div class="progress-bar"><i style="width:${(current / steps.length) * 100}%"></i></div>
    </div>
    <div class="wizard-content">
      ${state.step === 0 ? renderCheckStep(state, t) : ""}
      ${state.step === 1 ? renderPrepareStep(state, t) : ""}
      ${state.step === 2 ? renderFinishStep(state, t) : ""}
    </div>`;
}

function wizardSteps(state: ViewState, t: Translate): string[] {
  return state.mode === "setup"
    ? [t("stepCheck"), t("stepPrepare"), t("stepFinish")]
    : [t("stepDiagnose"), t("stepRepair"), t("stepVerify")];
}

// ---------------------------------------------------------------- check step

function renderCheckStep(state: ViewState, t: Translate): string {
  if (state.busy && !state.report) return renderChecklist(state, t);
  const snapshot = state.report?.snapshot;
  if (!snapshot) {
    if (state.checkError) {
      return `${resultBanner("bad", t("checkFailed"), state.checkError, t)}
        <div class="wizard-actions"><button id="run-check" class="button primary">${t("checkRetry")}</button></div>`;
    }
    return `<article class="panel intro-panel">
      <div class="intro-icon" aria-hidden="true">${searchIcon()}</div>
      <p class="intro-text">${t("checkIntro")}</p>
      <button id="run-check" class="button primary">${t("checkStart")}</button>
    </article>`;
  }
  const issues = checkIssues(snapshot, state.profile);
  const ready = issues.length === 0;
  const repair = state.mode === "repair";
  const verdict = repair
    ? ready
      ? resultBanner("good", t("repairHealthy"), t("repairHealthyBody"), t)
      : resultBanner("warn", t("verdictRepair"), t("verdictRepairBody"), t)
    : ready
      ? resultBanner("good", t("verdictReady"), t("verdictReadyBody"), t)
      : resultBanner("info", t("verdictFresh"), t("verdictFreshBody"), t);
  const cta = repair
    ? ready ? t("toVerify") : t("continueRepair")
    : ready ? t("toVerify") : t("continue");
  return `
    ${verdict}
    ${repair && !ready ? `<ul class="fix-list">${issues.map((key) => `<li>${arrowIcon()}<span>${escapeHtml(t(key, { port: state.profile.ssh.port }))}</span></li>`).join("")}</ul>` : ""}
    ${renderFacts(snapshot, t)}
    <div class="wizard-actions">
      <button id="run-check" class="button secondary">${t("checkRetry")}</button>
      <button id="check-continue" class="button primary">${cta}</button>
    </div>`;
}

// Paced checklist shown while the probe runs: the backend reports one shot,
// so items light up on a staggered CSS animation as a "working" indicator.
function renderChecklist(state: ViewState, t: Translate): string {
  const items: MessageKey[] = ["checkItemSystem", "checkItemSsh", "checkItemNetwork", "checkItemPermission"];
  return `<article class="panel checklist-panel" aria-busy="true">
    <h2>${t("checkingTitle")}</h2>
    <ol class="check-list">${items.map((key, index) => `<li style="--i:${index}"><span class="check-state" aria-hidden="true"></span>${t(key)}</li>`).join("")}</ol>
  </article>`;
}

function renderFacts(snapshot: Snapshot, t: Translate): string {
  const network = !snapshot.tailscale.installed
    ? t("factNetworkMissing")
    : snapshot.tailscale.online
      ? `${t("factNetworkOnline")}${snapshot.tailscale.ip ? ` · ${escapeHtml(snapshot.tailscale.ip)}` : ""}`
      : t("factNetworkOffline");
  const service = snapshot.sshService.running ? t("factServiceRunning") : t("factServiceStopped");
  const permission = snapshot.isAdministrator ? t("factPermissionAdmin") : t("factPermissionStandard");
  return `<details class="facts"><summary>${t("factsTitle")}</summary>
    <dl class="fact-table">
      <div><dt>${t("factHost")}</dt><dd>${escapeHtml(snapshot.hostname)}</dd></div>
      <div><dt>${t("factPlatform")}</dt><dd>${escapeHtml(snapshot.platform)} · ${escapeHtml(snapshot.arch)}</dd></div>
      <div><dt>${t("factPermission")}</dt><dd>${permission}</dd></div>
      <div><dt>${t("factNetwork")}</dt><dd>${network}</dd></div>
      <div><dt>${t("factService")}</dt><dd>${service}</dd></div>
    </dl>
  </details>`;
}

// ---------------------------------------------------------------- prepare step

function renderPrepareStep(state: ViewState, t: Translate): string {
  if (state.installState === "waiting-for-permission" || state.installState === "running") {
    return renderInstallProgress(state, t);
  }
  const plan = state.planReport?.plan;
  const blockers = plan?.blockers ?? [];
  const selected = state.selectedKey?.publicKey ?? state.profile.ssh.publicKeys[0] ?? "";
  const keyNeeded = state.mode === "setup"
    || Boolean(plan && (plan.actions.some((action) => action.operation === "configure_keys") || blockers.some((blocker) => /public key/i.test(blocker))));
  const keyReady = !keyNeeded || Boolean(selected);
  return `
    ${state.profile.labels.cardDisplayName ? cardLoadedNote(state, t) : ""}
    ${renderPrepareOutcome(state, t, plan, keyNeeded, keyReady, selected)}`;
}

function renderPrepareOutcome(
  state: ViewState, t: Translate,
  plan: Report["plan"], keyNeeded: boolean, keyReady: boolean, selected: string
): string {
  if (state.busy && !plan) {
    return `<article class="panel intro-panel"><span class="spinner" aria-hidden="true"></span><p class="intro-text">${t("prepareLoading")}</p></article>`;
  }
  if (state.planError) {
    return `${resultBanner("bad", t("prepareFailed"), state.planError, t)}
      <div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("backStep")}</button><button id="plan-retry" class="button primary">${t("prepareRetry")}</button></div>`;
  }
  if (!plan) return "";
  const blockers = plan.blockers ?? [];
  if (state.installState === "failed") {
    return `${resultBanner("bad", t("installFailed"), state.installError || t("installFailedBody"), t)}
      ${renderChangeList(state, t, plan)}
      ${planActionsRow(state, t, plan, keyReady, true)}`;
  }
  if (state.installState === "cancelled") {
    return `${resultBanner("info", t("installCancelled"), "", t)}
      ${renderPrepareBody(state, t, plan, keyNeeded, keyReady, selected, true)}`;
  }
  if (blockers.length > 0) {
    return renderBlockedPrepare(state, t, plan, keyNeeded, selected, blockers);
  }
  if (plan.noChanges) {
    return `${resultBanner("good", t("noChangesTitle"), t("noChangesBody"), t)}
      <div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("backStep")}</button><button id="test-now" class="button primary">${t("toVerify")}</button></div>`;
  }
  return renderPrepareBody(state, t, plan, keyNeeded, keyReady, selected, false);
}

function renderBlockedPrepare(
  state: ViewState, t: Translate, plan: NonNullable<Report["plan"]>,
  keyNeeded: boolean, selected: string, blockers: string[]
): string {
  return `
    ${resultBanner("bad", t("prepareBlocked"), blockers.join(" "), t)}
    <div class="summary-rows">
      ${renderNetworkSummary(state, t)}
      ${keyNeeded ? renderKeySummary(state, t, selected) : ""}
    </div>
    ${state.showNetwork ? renderNetworkChoice(state, t) : ""}
    ${keyNeeded && state.showKey ? renderKeyPicker(state, t, selected) : ""}
    ${renderChangeList(state, t, plan)}
    <div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("backStep")}</button></div>`;
}

function renderPrepareBody(
  state: ViewState, t: Translate, plan: NonNullable<Report["plan"]>,
  keyNeeded: boolean, keyReady: boolean, selected: string, retry: boolean
): string {
  const tailnet = state.profile.transport.mode === "tailnet";
  const repairing = state.mode === "repair";
  const verdict = resultBanner(
    "good",
    repairing ? t("repairReady") : t("prepareReady"),
    repairing
      ? tailnet ? t("repairReadyTailnet") : t("repairReadyLan")
      : tailnet ? t("prepareReadyTailnet") : t("prepareReadyLan"),
    t
  );
  return `
    ${verdict}
    <div class="summary-rows">
      ${renderNetworkSummary(state, t, tailnet)}
      ${keyNeeded ? renderKeySummary(state, t, selected) : ""}
    </div>
    ${state.showNetwork ? renderNetworkChoice(state, t) : ""}
    ${keyNeeded && state.showKey ? renderKeyPicker(state, t, selected) : ""}
    ${renderChangeList(state, t, plan)}
    ${planActionsRow(state, t, plan, keyReady, retry)}`;
}

function renderNetworkSummary(state: ViewState, t: Translate, tailnet = state.profile.transport.mode === "tailnet"): string {
  const online = state.report?.snapshot?.tailscale.online ?? false;
  const mode = state.profile.transport.mode;
  const value = tailnet
    ? online ? t("summaryNetworkTailnet") : t("summaryNetworkTailnetOffline")
    : mode === "lan" ? t("summaryNetworkLan")
      : mode === "none" ? t("summaryNetworkNone")
        : t("summaryNetworkCustom");
  const canChange = mode === "tailnet" || mode === "lan";
  return `<div class="summary-row">
    <span class="summary-icon">${networkIcon()}</span>
    <div class="summary-text"><small>${t("summaryNetwork")}</small><b>${value}</b></div>
    ${canChange ? `<button id="change-network" class="change-link" aria-expanded="${state.showNetwork}">${t("changeLink")}</button>` : ""}
  </div>`;
}

function renderKeySummary(state: ViewState, t: Translate, selected: string): string {
  const label = state.selectedKey?.label ?? "";
  const value = selected ? `${escapeHtml(label)} · ${escapeHtml(fingerprintPreview(selected))}` : "—";
  return `<div class="summary-row">
    <span class="summary-icon">${keyIcon()}</span>
    <div class="summary-text"><small>${t("summaryKey")}</small><b>${value}</b></div>
    <button id="change-key" class="change-link" aria-expanded="${state.showKey}">${t("changeLink")}</button>
  </div>`;
}

function renderNetworkChoice(state: ViewState, t: Translate): string {
  const snapshot = state.report?.snapshot;
  const mode = state.profile.transport.mode === "lan" ? "lan" : "tailnet";
  const tailscaleNote = !snapshot?.tailscale.installed ? t("tailscaleMissing") : !snapshot.tailscale.online ? t("tailscaleOffline") : "";
  return `
    <article class="panel choice-panel">
      <div class="choice-grid">
        <label class="choice-card ${mode === "tailnet" ? "selected" : ""}"><input type="radio" name="network-mode" value="tailnet" ${mode === "tailnet" ? "checked" : ""} ${state.busy ? "disabled" : ""}/><span class="choice-body"><b>${t("netTailscale")}</b><small>${t("netTailscaleBody")}</small></span></label>
        <label class="choice-card ${mode === "lan" ? "selected" : ""}"><input type="radio" name="network-mode" value="lan" ${mode === "lan" ? "checked" : ""} ${state.busy ? "disabled" : ""}/><span class="choice-body"><b>${t("netLan")}</b><small>${t("netLanBody")}</small></span></label>
      </div>
      ${tailscaleNote && mode === "tailnet" ? `<aside class="info-note">${infoIcon()}<span>${tailscaleNote}</span></aside>` : ""}
    </article>`;
}

function renderKeyPicker(state: ViewState, t: Translate, selected: string): string {
  const neutral = !selected && !state.keyAttempted;
  return `
    <article class="panel key-panel">
      ${!selected ? `
        <div class="key-missing ${neutral ? "" : "error"}">
          <strong>${t("keyMissingTitle")}</strong>
          <p>${neutral ? t("keyMissingBody") : t("keyInvalid")}</p>
        </div>` : ""}
      ${state.detectedKeys.length ? `<div class="detected-keys"><strong>${t("foundKeys")}</strong><p class="muted">${t("foundKeysNote")}</p>${state.detectedKeys.map((key, index) => `<label class="key-option"><input type="radio" name="controller-key" value="${index}" ${key.publicKey === selected ? "checked" : ""}/><span><b>${escapeHtml(key.label)}</b><small>${escapeHtml(fingerprintPreview(key.publicKey))}</small></span></label>`).join("")}</div>` : ""}
      <label class="field"><span>${t("pasteKey")}</span><textarea id="public-key" rows="2" placeholder="${t("pastePlaceholder")}" ${state.busy ? "disabled" : ""}>${escapeHtml(selected)}</textarea></label>
      <div class="key-actions"><button id="import-key" class="button secondary" ${state.busy ? "disabled" : ""}>${t("importKey")}</button><button id="generate-key" class="button secondary" ${state.busy ? "disabled" : ""}>${t("generateKey")}</button>${selected ? `<button id="export-pairing" class="button ghost" ${state.busy ? "disabled" : ""}>${t("exportPairing")}</button>` : ""}</div>
      ${state.detectedKeys.length === 0 ? `<p class="small-note">${t("generateNote")}</p>` : ""}
      ${selected ? `<p class="success-note">${checkIcon()} ${t("keySelected")} · ${escapeHtml(fingerprintPreview(selected))}</p>` : ""}
    </article>`;
}

function renderChangeList(state: ViewState, t: Translate, plan: NonNullable<Report["plan"]>): string {
  const actions = plan.actions;
  const selfCut = actions.some((action) => action.selfCutRisk);
  if (actions.length === 0 && !selfCut) return "";
  return `
    <section class="change-list" aria-label="${t("whatHappens")}">
      <h2>${t("whatHappens")}</h2>
      <ol>${actions.map((action) => `<li><span class="change-icon">${humanActionIcon(action)}</span><span>${escapeHtml(humanActionLabel(action, state, t))}</span></li>`).join("")}</ol>
      ${selfCut ? `<aside class="danger-note">${warningIcon()}<span>${t("errorSelfCut")}</span></aside>` : ""}
    </section>`;
}

function planActionsRow(state: ViewState, t: Translate, plan: NonNullable<Report["plan"]>, keyReady: boolean, retry: boolean): string {
  const blockers = plan.blockers ?? [];
  const disabled = state.busy || !keyReady;
  const label = state.mode === "repair"
    ? (retry ? t("repairRetry") : t("repairAction"))
    : (retry ? t("installRetry") : t("installAction"));
  return `<div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("backStep")}</button>${blockers.length ? "" : `<button id="open-install" class="button primary" ${disabled ? "disabled" : ""}>${label}</button>`}</div>`;
}

// ------------------------------------------------------- install progress

function renderInstallProgress(state: ViewState, t: Translate): string {
  const waiting = state.installState === "waiting-for-permission";
  const actions = state.planReport?.plan?.actions ?? [];
  const repairing = state.mode === "repair";
  return `
    ${resultBanner("info", waiting ? t("waitingUacTitle") : repairing ? t("repairingTitle") : t("installingTitle"), waiting ? t("waitingUacBody") : repairing ? t("repairingBody") : t("installingBody"), t)}
    <ol class="install-list">${actions.map((action) => {
      const status = actionStatus(state.progress, action.id);
      return `<li class="${status}"><span class="install-state" aria-hidden="true">${status === "done" ? checkIcon() : status === "running" ? `<i class="spinner-mini"></i>` : ""}</span><span>${escapeHtml(humanActionLabel(action, state, t))}</span><small>${status === "done" ? t("stateDone") : status === "running" ? t("stateRunning") : t("stateWaiting")}</small></li>`;
    }).join("")}</ol>`;
}

function actionStatus(progress: Array<{ kind: string; actionId?: string }>, actionId: string): "pending" | "running" | "done" {
  let status: "pending" | "running" | "done" = "pending";
  for (const event of progress) {
    if (event.actionId !== actionId) continue;
    if (event.kind === "started") status = "running";
    if (event.kind === "completed") status = "done";
  }
  return status;
}

// ---------------------------------------------------------------- finish step

function renderFinishStep(state: ViewState, t: Translate): string {
  if (state.busy) {
    return `<article class="panel intro-panel"><span class="spinner" aria-hidden="true"></span><p class="intro-text">${t("verifyRetry")}…</p></article>`;
  }
  if (state.verifyError) {
    return `${resultBanner("bad", t("verifyFailed"), state.verifyError, t)}
      <div class="wizard-actions"><button id="verify-again" class="button primary">${t("verifyRetry")}</button></div>`;
  }
  const report = state.verifyReport;
  const snapshot = report?.snapshot;
  const remaining = (report?.plan?.actions.length ?? 0) + (report?.plan?.blockers?.length ?? 0) || (report?.success ? 0 : 1);
  const ready = Boolean(report?.success && remaining === 0);
  const address = state.profile.transport.mode === "tailnet" ? snapshot?.tailscale.ip : snapshot?.network.lanIps?.[0];
  const host = address || snapshot?.hostname || "HOST";
  const user = snapshot?.targetUser || "user";
  const command = `ssh -p ${state.profile.ssh.port} ${user}@${host}`;
  if (!ready) {
    return `
      ${resultBanner("warn", t("verifyPending"), t("verifyPendingBody"), t)}
      <div class="wizard-actions"><button id="verify-again" class="button secondary">${t("verifyRetry")}</button><button id="finish" class="button primary">${t("finishAction")}</button></div>`;
  }
  return `
    <section class="done-hero">
      <span class="done-icon" aria-hidden="true">${monitorCheckIcon()}</span>
      <h1>${t("doneTitle")}</h1>
      <p>${t("doneBody")}</p>
      <div class="command-box"><code>${escapeHtml(command)}</code><button id="copy-command" class="button primary">${copyIcon()} ${t("copyCommand")}</button></div>
      <p class="done-facts">${t("doneFacts", { host: escapeHtml(snapshot?.hostname ?? "—"), address: escapeHtml(address ?? "—"), port: state.profile.ssh.port })}</p>
    </section>
    <section class="next-steps">
      <h2>${t("firstConnectTitle")}</h2>
      <ol>
        <li>${t("firstConnect1")}</li>
        <li>${t("firstConnect2")}</li>
        <li>${t("firstConnect3")}</li>
      </ol>
    </section>
    <div class="wizard-actions"><button id="finish" class="button primary">${t("finishAction")}</button></div>`;
}

// ---------------------------------------------------------------- advanced

export function renderAdvanced(state: ViewState, t: Translate): string {
  const profile = state.profile;
  return `
    <div class="wizard-header"><button class="text-button" id="advanced-back">${backIcon()} ${t("backHome")}</button><div><p class="eyebrow">${t("homeAdvancedLink")}</p><h1>${t("advancedTitle")}</h1><p class="muted">${t("advancedLead")}</p></div></div>

    <section class="panel"><h2>${t("groupSystem")}</h2>
      <div class="form-grid">
        ${selectField("target-platform", t("targetPlatform"), profile.target.platform, [["auto", t("optionAuto")], ["windows", t("optionWindows")], ["linux", t("optionLinux")], ["macos", t("optionMacOS")], ["wsl", t("optionWSL")]])}
        ${numberField("ssh-port", t("sshPort"), profile.ssh.port, 1, 65535)}
        ${selectField("transport-mode", t("transport"), profile.transport.mode, [["tailnet", t("optionTailnet")], ["lan", t("optionLan")], ["custom", t("optionCustom")], ["none", t("optionNone")]])}
        ${selectField("exposure-mode", t("exposure"), profile.exposure.mode, [["tailnet", t("optionTailnet")], ["lan", t("optionLan")], ["custom", t("optionCustom")], ["none", t("optionNone")]])}
        ${selectField("download-strategy", t("downloadSource"), profile.download.strategy, [["official", t("optionOfficial")], ["package-manager", t("optionPackageManager")], ["mirror", t("optionMirror")], ["proxy", t("optionProxy")], ["offline", t("optionOffline")], ["cache", t("optionCache")]])}
      </div>
      <label class="check-row"><input id="prevent-self-cut" type="checkbox" ${profile.safety.preventSelfCut ? "checked" : ""}/><span>${t("preventSelfCut")}</span></label>
      <label class="check-row"><input id="auto-rollback" type="checkbox" ${profile.safety.autoRollback ? "checked" : ""}/><span>${t("autoRollback")}</span></label>
    </section>

    <section class="panel"><h2>${t("groupKeys")}</h2>
      <label class="field"><span>${t("publicKeys")}</span><textarea id="advanced-keys" rows="4">${escapeHtml(profile.ssh.publicKeys.join("\n"))}</textarea></label>
      <div class="toolbar"><button id="import-profile" class="button secondary">${uploadIcon()} ${t("importProfile")}</button><button id="export-profile" class="button secondary">${downloadIcon()} ${t("exportProfile")}</button></div>
      <hr class="group-sep"/>
      <h3>${t("cardTitle")}</h3><p class="muted">${t("cardLead")}</p>
      <div class="form-grid">
        <label class="field"><span>${t("cardName")}</span><input id="card-display-name" type="text" maxlength="128" value="${escapeHtml(state.personalCard.displayName)}" placeholder="${t("cardNamePlaceholder")}"/></label>
        <label class="field"><span>${t("cardControllerName")}</span><input id="card-controller-name" type="text" maxlength="128" value="${escapeHtml(state.personalCard.controllerName)}" placeholder="${t("cardControllerPlaceholder")}"/></label>
      </div>
      <label class="field"><span>${t("cardNote")}</span><textarea id="card-note" rows="2" maxlength="1024" placeholder="${t("cardNotePlaceholder")}">${escapeHtml(state.personalCard.note)}</textarea></label>
      <label class="field"><span>${t("cardAuthKey")}</span><input id="card-tailscale-auth-key" type="password" autocomplete="off" spellcheck="false" value="${escapeHtml(profile.transport.authKey ?? "")}" placeholder="tskey-auth-…"/><small>${t("cardAuthKeyHint")}</small></label>
      <p class="small-note">${t("cardIncludes")}</p>
      <div class="toolbar"><button id="import-personal-card-advanced" class="button secondary">${uploadIcon()} ${t("cardImport")}</button><button id="export-personal-card" class="button secondary">${downloadIcon()} ${t("cardExport")}</button></div>
    </section>

    <section class="panel"><h2>${t("groupDiagnostics")}</h2>
      <div class="toolbar"><button id="advanced-check" class="button secondary">${t("runCheck")}</button><button id="advanced-plan" class="button secondary">${t("buildPlan")}</button><button id="export-report-advanced" class="button secondary">${t("exportReport")}</button></div>
      <p class="small-note">${t("reportPrivacy")}</p>
      ${state.report ? technicalDetails(state.report, t) : ""}
    </section>

    <section class="panel"><h2>${t("groupRecovery")}</h2>
      <p class="muted">${t("recoveryNote")}</p>
      <div class="toolbar"><button id="rollback-last" class="button secondary" ${state.report?.journalPath ? "" : "disabled"}>${t("rollbackLast")}</button><button id="check-update" class="button secondary">${t("updateCheck")}</button></div>
      <p class="small-note">${t("unsignedNotice")}</p>
      <div class="panel-footer"><span id="advanced-status">${t("autoApplyNote")}</span></div>
    </section>`;
}

// ------------------------------------------------------------- confirm dialog

export function renderConfirmActions(state: ViewState, t: Translate): string {
  const actions = state.planReport?.plan?.actions ?? [];
  return actions.map((action) => `<div><span>${humanActionIcon(action)}</span><div><strong>${escapeHtml(humanActionLabel(action, state, t))}</strong></div></div>`).join("");
}

export function confirmAckKey(state: ViewState): MessageKey {
  return state.profile.transport.mode === "lan" ? "confirmAckLan" : "confirmAckTailnet";
}

// ---------------------------------------------------------------- shared bits

function resultBanner(kind: "good" | "info" | "warn" | "bad", title: string, body: string, t: Translate): string {
  const icon = kind === "good" ? checkIcon() : kind === "bad" ? closeIcon() : kind === "warn" ? warningIcon() : infoIcon();
  return `<section class="result-banner ${kind}" role="status"><span>${icon}</span><div><h2>${escapeHtml(title)}</h2>${body ? `<p>${escapeHtml(body)}</p>` : ""}</div></section>`;
}

function cardLoadedNote(state: ViewState, t: Translate): string {
  return `<aside class="info-note card-loaded">${keyIcon()}<div><strong>${escapeHtml(t("cardLoaded", { name: state.profile.labels.cardDisplayName ?? "" }))}</strong>${state.profile.labels.cardNote ? `<p>${escapeHtml(state.profile.labels.cardNote)}</p>` : ""}<small>${state.profile.transport.authKey ? t("cardAuthKeyLoaded") : t("cardIncludes")}</small></div></aside>`;
}

function technicalDetails(report: Report | undefined, t: Translate): string {
  if (!report) return "";
  return `<details class="technical-details"><summary>${t("technicalDetails")}</summary><div class="detail-grid">${report.snapshot ? `<div><b>${t("snapshotData")}</b><pre>${escapeHtml(JSON.stringify(report.snapshot, null, 2))}</pre></div>` : ""}${report.plan ? `<div><b>${t("planData")}</b><pre>${escapeHtml(JSON.stringify(report.plan, null, 2))}</pre></div>` : ""}</div></details>`;
}

function humanActionLabel(action: PlanAction, state: ViewState, t: Translate): string {
  if (action.operation === "configure_firewall") {
    return state.profile.transport.mode === "lan" ? t("actionFirewallLan") : t("actionFirewall");
  }
  if (action.operation === "authenticate_tailscale") {
    return state.profile.transport.authKey ? t("actionTailscaleAuth") : t("actionTailscaleConnect");
  }
  const keys: Record<string, MessageKey> = {
    install_ssh: "actionInstall",
    configure_sshd: "actionConfig",
    configure_keys: "actionKeys",
    enable_sshd: "actionService",
    install_tailscale: "actionTailscaleInstall"
  };
  const key = keys[action.operation];
  return key ? t(key, { port: state.profile.ssh.port }) : t("actionManual");
}

function humanActionIcon(action: PlanAction): string {
  if (action.layer === "firewall") return shieldIcon();
  if (action.layer === "authentication") return keyIcon();
  if (action.layer === "transport") return networkIcon();
  if (action.layer === "ssh-service") return powerIcon();
  return packageIcon();
}

// Missing items, expressed as change-list lines (reused by repair mode).
export function checkIssues(snapshot: Snapshot, profile: Profile): MessageKey[] {
  const issues: MessageKey[] = [];
  if (!snapshot.sshServer.installed || !snapshot.sshService.installed) issues.push("actionInstall");
  if (
    snapshot.sshPort !== profile.ssh.port || snapshot.sshPort === 0 || !snapshot.sshConfigValid
    || !snapshot.sshAuthenticationChecked || snapshot.sshPasswordAuthentication !== profile.ssh.passwordAuthentication
    || snapshot.sshKbdInteractiveAuthentication || !snapshot.sshPubkeyAuthentication
  ) issues.push("actionConfig");

  const keysReady = profile.ssh.passwordAuthentication
    ? true
    : snapshot.authorizedKeysChecked && (profile.ssh.publicKeys.length > 0
      ? snapshot.authorizedKeysMatch
      : snapshot.authorizedKeysCount > 0);
  if (!keysReady) issues.push("actionKeys");
  if (!snapshot.sshService.running || snapshot.sshService.startPolicy === "disabled") issues.push("actionService");

  if (profile.exposure.mode !== "none" && !firewallMatchesProfile(snapshot, profile)) {
    issues.push(profile.transport.mode === "lan" ? "actionFirewallLan" : "actionFirewall");
  }
  if (profile.transport.mode === "tailnet" && !snapshot.tailscale.online) issues.push("actionTailscaleConnect");
  return issues;
}

function firewallMatchesProfile(snapshot: Snapshot, profile: Profile): boolean {
  const firewall = snapshot.firewall;
  const providerSupported = snapshot.platform === "windows"
    ? firewall.provider === "windows-firewall"
    : snapshot.platform === "linux" || snapshot.platform === "wsl"
      ? firewall.provider === "ufw" || firewall.provider === "firewall-cmd"
      : snapshot.platform === "macos" && firewall.provider === "application-firewall";
  if (!providerSupported || !firewall.checked || !firewall.enabled || firewall.broadExposure
    || (firewall.unresolvedBroadRules?.length ?? 0) > 0
    || (firewall.portRangeRules?.length ?? 0) > 0
    || !(firewall.ports ?? []).includes(profile.ssh.port)) return false;

  const desired = profile.exposure.mode === "tailnet"
    ? ["100.64.0.0/10", "fd7a:115c:a1e0::/48"]
    : profile.exposure.mode === "lan"
      ? snapshot.platform === "windows" ? ["localsubnet"] : (snapshot.network.lanScopes ?? [])
      : profile.exposure.mode === "custom" ? profile.exposure.customCidrs : [];
  if (desired.length === 0) return false;

  const normalize = (values: string[]) => new Set(values.flatMap((value) => value.split(/[\s,]+/))
    .map((value) => value.trim().toLowerCase())
    .filter(Boolean));
  const existing = normalize(firewall.scopes ?? []);
  const wanted = normalize(desired);
  if (existing.size !== wanted.size) return false;
  for (const scope of wanted) if (!existing.has(scope)) return false;
  return true;
}

function fingerprintPreview(key: string): string {
  const parts = key.split(/\s+/);
  const data = parts[1] ?? "";
  return `${parts[0] ?? "public-key"} · ${data.slice(0, 12)}…${data.slice(-8)}`;
}

function selectField(id: string, label: string, value: string, options: Array<[string, string]>): string {
  return `<label class="field"><span>${label}</span><select id="${id}">${options.map(([key, text]) => `<option value="${key}" ${key === value ? "selected" : ""}>${text}</option>`).join("")}</select></label>`;
}

function numberField(id: string, label: string, value: number, min: number, max: number): string {
  return `<label class="field"><span>${label}</span><input id="${id}" type="number" value="${value}" min="${min}" max="${max}"/></label>`;
}

export function simpleEvent(language: Language, event: { kind: string; message: string; actionId?: string }): string {
  if (event.kind === "started") return language === "zh-CN" ? "正在处理" : "In progress";
  if (event.kind === "completed") return language === "zh-CN" ? "已完成" : "Completed";
  if (event.kind === "error") return language === "zh-CN" ? "没有完成，已停止后续步骤" : "Did not finish; later steps stopped";
  return event.message;
}
