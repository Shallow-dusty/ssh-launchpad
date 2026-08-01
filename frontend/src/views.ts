import type { Language, MessageKey } from "./i18n";
import { escapeHtml } from "./browser-utils";
import {
  arrowIcon, backIcon, checkIcon, closeIcon, computerIcon, copyIcon, devicesIcon,
  doorIcon, downloadIcon, infoIcon, keyIcon, lockIcon, monitorCheckIcon, networkIcon,
  packageIcon, powerIcon, repairIcon, screenIcon, searchIcon, shieldIcon, slidersIcon,
  uploadIcon, userIcon, warningIcon
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
}

export function renderHome(state: ViewState, t: Translate): string {
  return `
    <div class="home">
      <section class="welcome"><p class="eyebrow">${t("version")}</p><h1>${t("homeTitle")}</h1><p class="lead">${t("homeLead")}</p></section>
      <section class="role-card material" aria-labelledby="role-title">
        <div class="role-icon" aria-hidden="true">${devicesIcon()}</div>
        <div><h2 id="role-title">${t("roleTitle")}</h2><div class="role-pair"><span><b>1</b>${t("roleTarget")}</span><i aria-hidden="true">${arrowIcon()}</i><span><b>2</b>${t("roleController")}</span></div><p>${t("roleBody")}</p></div>
      </section>
      <section class="personal-card-strip material" aria-labelledby="personal-card-title">
        <span class="personal-card-icon" aria-hidden="true">${keyIcon()}</span>
        <div><h2 id="personal-card-title">${t("personalCardTitle")}</h2><p>${t("personalCardLead")}</p></div>
        <div class="toolbar"><button id="import-personal-card" class="button primary">${uploadIcon()} ${t("importPersonalCard")}</button><button id="create-personal-card" class="button secondary">${t("createPersonalCard")}</button></div>
      </section>
      <div class="task-grid">
        ${taskCard("setup", t("taskSetup"), t("taskSetupBody"), screenIcon(), true, t)}
        ${taskCard("repair", t("taskRepair"), t("taskRepairBody"), repairIcon(), false, t)}
        ${taskCard("advanced", t("taskAdvanced"), t("taskAdvancedBody"), slidersIcon(), false, t)}
      </div>
      <footer class="privacy-foot"><span>${lockIcon()}</span>${t("noTelemetry")}</footer>
    </div>`;
}

function taskCard(id: string, title: string, body: string, icon: string, recommended: boolean, t: Translate): string {
  return `<button class="task-card material ${recommended ? "recommended" : ""}" data-task="${id}">${recommended ? `<span class="recommended">${t("recommended")}</span>` : ""}<span class="task-icon">${icon}</span><strong>${title}</strong><p>${body}</p><span class="task-arrow">${arrowIcon()}</span></button>`;
}

export function renderWizard(state: ViewState, t: Translate): string {
  const steps = wizardSteps(state, t);
  return `
    <div class="wizard-header">
      <button class="text-button" id="wizard-back">${backIcon()} ${t("backHome")}</button>
      <div><p class="eyebrow">${state.mode === "setup" ? t("wizardSetup") : t("wizardRepair")}</p><h1>${steps[state.step] ?? steps[0]}</h1></div>
    </div>
    <ol class="stepper" aria-label="${state.mode === "setup" ? t("wizardSetup") : t("wizardRepair")}">${steps.map((label, index) => `<li class="${index === state.step ? "active" : ""} ${index < state.step ? "done" : ""}" aria-current="${index === state.step ? "step" : "false"}"><span>${index < state.step ? checkIcon() : index + 1}</span><b>${label}</b></li>`).join("")}</ol>
    <div class="wizard-content">
      ${state.step === 0 ? renderCheckStep(state, t) : ""}
      ${state.step === 1 ? renderPlanStep(state, t) : ""}
      ${state.step === 2 ? renderTestStep(state, t) : ""}
    </div>`;
}

function wizardSteps(state: ViewState, t: Translate): string[] {
  return state.mode === "setup"
    ? [t("stepCheck"), t("stepPlan"), t("stepDone")]
    : [t("stepDiagnose"), t("stepRepair"), t("stepVerify")];
}

function renderCheckStep(state: ViewState, t: Translate): string {
  if (state.busy && !state.report) return `<article class="focus-card material loading-card"><span class="spinner" aria-hidden="true"></span><h2>${t("checking")}</h2><p>${t("checkBody")}</p></article>`;
  const snapshot = state.report?.snapshot;
  if (!snapshot) {
    if (state.checkError) {
      return `${resultBanner("bad", t("checkFailed"), state.checkError, t)}<article class="focus-card material"><div class="large-symbol">${searchIcon()}</div><button id="run-check" class="button primary">${t("checkNow")}</button></article>`;
    }
    return `<article class="focus-card material"><div class="large-symbol">${searchIcon()}</div><h2>${t("stepCheck")}</h2><p>${t("checkBody")}</p><button id="run-check" class="button primary">${t("checkNow")}</button></article>`;
  }
  const issues = checkIssues(snapshot, state.profile, t);
  const ready = issues.length === 0;
  return `
    ${state.mode === "repair" ? `<p class="muted repair-intro">${infoIcon()} ${t("repairIntro")}</p>` : ""}
    ${resultBanner(ready ? "good" : "info", ready ? t("ready") : t("missingSteps", { count: issues.length }), ready ? t("alreadyConfigured") : t("issuesLead"), t)}
    ${issues.length ? `<ul class="issue-list">${issues.map((issue) => `<li>${arrowIcon()}<span>${escapeHtml(issue)}</span></li>`).join("")}</ul>` : ""}
    <div class="plain-grid">
      ${plainCard(t("computer"), snapshot.hostname, `${snapshot.platform} · ${snapshot.arch}`, computerIcon())}
      ${plainCard(t("permission"), snapshot.isAdministrator ? t("administrator") : t("standardUser"), "", userIcon())}
      ${plainCard(t("secureNetwork"), snapshot.tailscale.online ? t("online") : t("unavailable"), snapshot.tailscale.ip ?? "", networkIcon())}
      ${plainCard(t("sshService"), snapshot.sshService.running ? t("running") : t("notRunning"), snapshot.sshService.name ?? "", powerIcon())}
    </div>
    ${technicalDetails(state.report, t)}
    <div class="wizard-actions"><button id="run-check" class="button secondary">${t("checkNow")}</button><button id="check-continue" class="button primary">${state.mode === "repair" ? t("continueFix") : t("continue")}</button></div>`;
}

function renderPlanStep(state: ViewState, t: Translate): string {
  if (state.installState === "waiting-for-permission" || state.installState === "running") {
    const waiting = state.installState === "waiting-for-permission";
    return `<article class="focus-card material loading-card"><span class="spinner" aria-hidden="true"></span><h2>${waiting ? t("waitingUAC") : t("installing")}</h2><p>${waiting ? t("waitingUACBody") : t("installingBody")}</p>${renderFriendlyProgress(state, t)}</article>`;
  }
  const plan = state.planReport?.plan;
  const blockers = plan?.blockers ?? [];
  const selected = state.selectedKey?.publicKey ?? state.profile.ssh.publicKeys[0] ?? "";
  // Repair mode only asks for a key when the plan actually needs one.
  const keyNeeded = state.mode === "setup"
    || Boolean(plan && (plan.actions.some((action) => action.operation === "configure_keys") || blockers.some((blocker) => /public key/i.test(blocker))));
  const keyReady = !keyNeeded || Boolean(selected);
  return `
    ${state.profile.labels.cardDisplayName ? cardLoadedNote(state, t) : ""}
    ${state.mode === "setup" ? renderNetworkChoice(state, t) : ""}
    ${keyNeeded ? renderKeyPicker(state, t, selected) : ""}
    ${renderPlanOutcome(state, t, keyReady)}`;
}

function cardLoadedNote(state: ViewState, t: Translate): string {
  return `<aside class="card-loaded-note material">${keyIcon()}<div><strong>${escapeHtml(t("cardLoaded", { name: state.profile.labels.cardDisplayName ?? "" }))}</strong>${state.profile.labels.cardNote ? `<p>${escapeHtml(state.profile.labels.cardNote)}</p>` : ""}<small>${state.profile.transport.authKey ? t("cardAuthKeyLoaded") : t("cardIncludesSettings")}</small></div></aside>`;
}

function renderNetworkChoice(state: ViewState, t: Translate): string {
  const snapshot = state.report?.snapshot;
  const mode = state.profile.transport.mode === "lan" ? "lan" : "tailnet";
  const tailscaleNote = !snapshot?.tailscale.installed ? t("tailscaleMissing") : !snapshot.tailscale.online ? t("tailscaleOffline") : "";
  return `
    <article class="panel material choice-panel">
      <div class="section-title"><div><h2>${t("networkTitle")}</h2></div>${networkIcon()}</div>
      <div class="choice-grid">
        <label class="choice-card ${mode === "tailnet" ? "selected" : ""}"><input type="radio" name="network-mode" value="tailnet" ${mode === "tailnet" ? "checked" : ""}/><span class="choice-body"><b>${t("netTailscale")}<em class="recommended">${t("recommended")}</em></b><small>${t("netTailscaleBody")}</small></span></label>
        <label class="choice-card ${mode === "lan" ? "selected" : ""}"><input type="radio" name="network-mode" value="lan" ${mode === "lan" ? "checked" : ""}/><span class="choice-body"><b>${t("netLan")}</b><small>${t("netLanBody")}</small></span></label>
      </div>
      ${tailscaleNote && mode === "tailnet" ? `<aside class="info-note">${infoIcon()}<span>${tailscaleNote}</span></aside>` : ""}
    </article>`;
}

function renderKeyPicker(state: ViewState, t: Translate, selected: string): string {
  return `
    <article class="key-card material">
      <div class="section-title"><div><h2>${t("keyTitle")}</h2><p>${t("keyExplain")}</p></div>${lockIcon()}</div>
      ${state.detectedKeys.length ? `<div class="detected-keys"><strong>${t("foundKeys")}</strong><p>${t("foundKeysWarn")}</p>${state.detectedKeys.map((key, index) => `<label class="key-option"><input type="radio" name="controller-key" value="${index}" ${key.publicKey === selected ? "checked" : ""}/><span><b>${escapeHtml(key.label)}</b><small>${escapeHtml(fingerprintPreview(key.publicKey))}</small></span></label>`).join("")}</div>` : ""}
      <label class="field"><span>${t("pasteKey")}</span><textarea id="public-key" rows="3" placeholder="${t("pastePlaceholder")}">${escapeHtml(selected)}</textarea></label>
      <div class="key-actions"><button id="import-key" class="button secondary">${t("importKey")}</button><button id="generate-key" class="button secondary">${t("generateKey")}</button>${selected ? `<button id="export-pairing" class="button ghost">${t("exportPairing")}</button>` : ""}</div>
      <p class="small-note">${t("generateExplain")}</p>
      ${selected ? `<p class="success-note">${checkIcon()} ${t("keySelected")}</p>` : ""}
      <p id="key-error" class="inline-error">${selected ? "" : t("keyRequired")}</p>
    </article>`;
}

function renderPlanOutcome(state: ViewState, t: Translate, keyReady: boolean): string {
  const plan = state.planReport?.plan;
  if (state.busy && !plan) return `<article class="focus-card material loading-card"><span class="spinner"></span><h2>${t("planLoading")}</h2><p>${t("planLead")}</p></article>`;
  if (state.planError) {
    return `${resultBanner("bad", t("planFailed"), state.planError, t)}<div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("backStep")}</button><button id="plan-retry" class="button primary">${t("retryPlan")}</button></div>`;
  }
  if (!plan) return "";
  const blockers = plan.blockers ?? [];
  if (state.installState === "failed") {
    return `${resultBanner("bad", t("installFailed"), state.installError || t("errorGeneric"), t)}${technicalDetails(state.activeJob?.report, t)}${renderPlanSummary(state, t, plan)}${planActionsRow(state, t, plan, keyReady, true)}`;
  }
  if (state.installState === "cancelled") {
    return `${resultBanner("info", t("noChanges"), t("cancelledUAC"), t)}${renderPlanSummary(state, t, plan)}${planActionsRow(state, t, plan, keyReady, true)}`;
  }
  if (blockers.length > 0) {
    return `${resultBanner("bad", t("planBlocked"), blockers.join(" "), t)}${renderPlanSummary(state, t, plan)}<div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("backStep")}</button></div>`;
  }
  if (plan.noChanges) {
    return `${resultBanner("good", t("ready"), t("alreadyConfigured"), t)}<div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("backStep")}</button><button id="test-now" class="button primary">${t("skipInstallVerify")}</button></div>`;
  }
  return `${renderPlanSummary(state, t, plan)}${planActionsRow(state, t, plan, keyReady, false)}`;
}

function renderPlanSummary(state: ViewState, t: Translate, plan: NonNullable<Report["plan"]>): string {
  const actions = plan.actions;
  const installs = actions.filter((action) => action.operation === "install_ssh" || action.operation === "install_tailscale").map((action) => humanActionLabel(action, t));
  const opensFirewall = actions.some((action) => action.operation === "configure_firewall");
  return `
    <article class="panel material">
      <div class="section-title"><div><h2>${t("planTitle")}</h2><p>${t("planLead")}</p></div><span class="count-pill">${t("actionCount", { count: actions.length })}</span></div>
      <div class="access-summary">
        <div><span>${packageIcon()}</span><small>${t("willInstall")}</small><b>${installs.length ? installs.join(" / ") : t("noChanges")}</b></div>
        <div><span>${doorIcon()}</span><small>${t("willOpen")}</small><b>${opensFirewall ? t("port", { port: state.profile.ssh.port }) : t("portNotOpened")}</b></div>
        <div><span>${userIcon()}</span><small>${t("whoCanConnect")}</small><b>${t("selectedController")}</b></div>
      </div>
      ${actions.some((action) => action.selfCutRisk) ? `<aside class="danger-note">${warningIcon()}<span>${t("selfCutNotice")}</span></aside>` : ""}
      <details class="technical-details"><summary>${t("actionDetails")}</summary><div class="human-actions">${actions.map((action) => humanAction(action, state, t)).join("")}</div><pre>${escapeHtml(JSON.stringify(actions, null, 2))}</pre></details>
    </article>`;
}

function planActionsRow(state: ViewState, t: Translate, plan: NonNullable<Report["plan"]>, keyReady: boolean, retry: boolean): string {
  const blockers = plan.blockers ?? [];
  const disabled = !keyReady || state.busy;
  return `<div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("backStep")}</button>${blockers.length ? "" : `<button id="open-install" class="button primary" ${disabled ? "disabled" : ""}>${retry ? t("retry") : t("safeInstall")}</button>`}</div>`;
}

export function renderTestStep(state: ViewState, t: Translate): string {
  if (state.busy) return `<article class="focus-card material loading-card"><span class="spinner"></span><h2>${t("testing")}</h2></article>`;
  if (state.verifyError) {
    return `${resultBanner("bad", t("verifyFailed"), state.verifyError, t)}<div class="wizard-actions"><button id="verify-again" class="button primary">${t("checkNow")}</button></div>`;
  }
  const report = state.verifyReport;
  const snapshot = report?.snapshot;
  const remaining = (report?.plan?.actions.length ?? 0) + (report?.plan?.blockers?.length ?? 0) || (report?.success ? 0 : 1);
  const ready = Boolean(report?.success && remaining === 0);
  const address = state.profile.transport.mode === "tailnet" ? snapshot?.tailscale.ip : snapshot?.network.lanIps?.[0];
  const host = address || snapshot?.hostname || "HOST";
  const user = snapshot?.targetUser || (state.language === "zh-CN" ? "你的用户名" : "YOUR_USER");
  const command = `ssh -p ${state.profile.ssh.port} ${user}@${host}`;
  return `
    ${resultBanner(ready ? "good" : "info", ready ? t("testReady") : t("testNeeds", { count: remaining }), ready ? t("testReadyBody") : t("testNeedsBody"), t)}
    <article class="connection-card material">
      <div class="connection-visual ${ready ? "ready" : ""}">${ready ? monitorCheckIcon() : devicesIcon()}<b>${ready ? t("visualReady") : t("visualWaiting")}</b></div>
      <h2>${t("connectFromOther")}</h2>
      <div class="connection-facts"><div><span>${t("host")}</span><strong>${escapeHtml(snapshot?.hostname ?? "—")}</strong></div><div><span>${t("address")}</span><strong>${escapeHtml(address ?? "—")}</strong></div><div><span>${t("portLabel")}</span><strong>${state.profile.ssh.port}</strong></div></div>
      <div class="copy-box"><code>${escapeHtml(command)}</code><button id="copy-command" class="button secondary">${copyIcon()} ${t("copyCommand")}</button></div>
      <aside class="info-note">${infoIcon()}<span>${t("nextDevice")}</span></aside>
      ${(report?.plan?.blockers?.length ?? 0) > 0 ? `<aside class="danger-note">${warningIcon()}<span>${escapeHtml(report!.plan!.blockers!.join(" "))}</span></aside>` : ""}
      ${technicalDetails(report, t)}
    </article>
    <div class="wizard-actions"><button id="verify-again" class="button secondary">${t("checkNow")}</button><button id="finish" class="button primary">${t("startOver")}</button></div>`;
}

export function renderAdvanced(state: ViewState, t: Translate): string {
  const profile = state.profile;
  return `
    <div class="wizard-header"><button class="text-button" id="advanced-back">${backIcon()} ${t("backHome")}</button><div><p class="eyebrow">${t("taskAdvanced")}</p><h1>${t("advancedTitle")}</h1><p>${t("advancedLead")}</p></div></div>
    <article class="panel material personal-card-panel">
      <div class="section-title"><div><h2>${t("personalCardTitle")}</h2><p>${t("personalCardLead")}</p></div>${keyIcon()}</div>
      <div class="form-grid">
        <label class="field"><span>${t("cardDisplayName")}</span><input id="card-display-name" type="text" maxlength="128" value="${escapeHtml(state.personalCard.displayName)}" placeholder="${t("cardDisplayNamePlaceholder")}"/></label>
        <label class="field"><span>${t("cardControllerName")}</span><input id="card-controller-name" type="text" maxlength="128" value="${escapeHtml(state.personalCard.controllerName)}" placeholder="${t("cardControllerNamePlaceholder")}"/></label>
      </div>
      <label class="field"><span>${t("cardNote")}</span><textarea id="card-note" rows="2" maxlength="1024" placeholder="${t("cardNotePlaceholder")}">${escapeHtml(state.personalCard.note)}</textarea></label>
      <label class="field"><span>${t("cardAuthKey")}</span><input id="card-tailscale-auth-key" type="password" autocomplete="off" spellcheck="false" value="${escapeHtml(profile.transport.authKey ?? "")}" placeholder="tskey-auth-…"/><small>${t("cardAuthKeyHint")}</small></label>
      <p class="small-note">${t("cardIncludesSettings")}</p>
      <div class="toolbar"><button id="import-personal-card-advanced" class="button secondary">${uploadIcon()} ${t("importPersonalCard")}</button><button id="export-personal-card" class="button primary">${downloadIcon()} ${t("exportPersonalCard")}</button></div>
    </article>
    <article class="panel material">
      <div class="toolbar"><button id="import-profile" class="button secondary">${uploadIcon()} ${t("importProfile")}</button><button id="export-profile" class="button secondary">${downloadIcon()} ${t("exportProfile")}</button><button id="advanced-check" class="button secondary">${t("runCheck")}</button><button id="advanced-plan" class="button primary">${t("buildPlan")}</button></div>
      <div class="form-grid">
        ${selectField("target-platform", t("targetPlatform"), profile.target.platform, [["auto", "Auto"], ["windows", "Windows"], ["linux", "Linux"], ["macos", "macOS"], ["wsl", "WSL"]])}
        ${numberField("ssh-port", t("sshPort"), profile.ssh.port, 1, 65535)}
        ${selectField("transport-mode", t("transport"), profile.transport.mode, [["tailnet", "Tailscale"], ["lan", "LAN"], ["custom", "Custom"], ["none", "None"]])}
        ${selectField("exposure-mode", t("exposure"), profile.exposure.mode, [["tailnet", "Tailnet only"], ["lan", "LAN"], ["custom", "Custom"], ["none", "None"]])}
        ${selectField("download-strategy", t("downloadSource"), profile.download.strategy, [["official", "Official"], ["package-manager", "Package manager"], ["mirror", "Mirror"], ["proxy", "Proxy"], ["offline", "Offline"], ["cache", "Cache"]])}
      </div>
      <label class="field"><span>${t("publicKeys")}</span><textarea id="advanced-keys" rows="5">${escapeHtml(profile.ssh.publicKeys.join("\n"))}</textarea></label>
      <label class="check-row"><input id="prevent-self-cut" type="checkbox" ${profile.safety.preventSelfCut ? "checked" : ""}/><span>${state.language === "zh-CN" ? "阻止可能切断当前远程连接的操作" : "Block changes that may cut the current remote connection"}</span></label>
      <label class="check-row"><input id="auto-rollback" type="checkbox" ${profile.safety.autoRollback ? "checked" : ""}/><span>${state.language === "zh-CN" ? "失败时自动恢复已完成的可恢复步骤" : "Automatically roll back completed reversible steps after failure"}</span></label>
      <div class="panel-footer"><span id="advanced-status">${t("advancedAutoApply")}</span></div>
      ${state.report ? technicalDetails(state.report, t) : ""}
    </article>
    <article class="panel material recovery-panel">
      <div><h2>${t("recoveryTitle")}</h2><p>${t("uninstallChoice")}</p></div>
      <div class="toolbar"><button id="rollback-last" class="button secondary" ${state.report?.journalPath ? "" : "disabled"}>${t("rollbackLast")}</button><button id="export-report-advanced" class="button secondary">${t("exportReport")}</button><button id="check-update" class="button secondary">${t("updateCheck")}</button></div>
      <p class="small-note">${t("reportPrivacy")}</p>
    </article>
    <aside class="info-note unsigned-note">${infoIcon()}<span>${t("unsignedNotice")}</span></aside>`;
}

export function renderConfirmActions(state: ViewState, t: Translate): string {
  const actions = state.planReport?.plan?.actions ?? [];
  return actions.map((action) => `<div><span>${humanActionIcon(action)}</span><div><strong>${humanActionLabel(action, t)}</strong><p>${humanReason(action, state, t)}</p></div></div>`).join("");
}

function resultBanner(kind: "good" | "info" | "warn" | "bad", title: string, body: string, t: Translate): string {
  const icon = kind === "good" ? checkIcon() : kind === "bad" ? closeIcon() : kind === "warn" ? warningIcon() : infoIcon();
  return `<section class="result-banner ${kind}" role="status"><span>${icon}</span><div><h2>${escapeHtml(title)}</h2><p>${escapeHtml(body)}</p></div></section>`;
}

function plainCard(label: string, value: string, note: string, icon: string): string {
  return `<article class="plain-card material"><span>${icon}</span><div><small>${label}</small><strong>${escapeHtml(value)}</strong>${note ? `<p>${escapeHtml(note)}</p>` : ""}</div></article>`;
}

function technicalDetails(report: Report | undefined, t: Translate): string {
  if (!report) return "";
  return `<details class="technical-details"><summary>${t("details")}</summary><div class="detail-grid">${report.snapshot ? `<div><b>${t("system")}</b><pre>${escapeHtml(JSON.stringify(report.snapshot, null, 2))}</pre></div>` : ""}${report.plan ? `<div><b>${t("rawReport")}</b><pre>${escapeHtml(JSON.stringify(report.plan, null, 2))}</pre></div>` : ""}</div></details>`;
}

function humanAction(action: PlanAction, state: ViewState, t: Translate): string {
  return `<div class="human-action"><span>${humanActionIcon(action)}</span><div><strong>${humanActionLabel(action, t)}</strong><p>${humanReason(action, state, t)}</p></div><b>${action.requiresElevation ? shieldIcon() : checkIcon()}</b></div>`;
}

function humanActionLabel(action: PlanAction, t: Translate): string {
  const keys: Record<string, MessageKey> = {
    install_ssh: "installSSH", configure_sshd: "configureSSH", configure_keys: "configureKeys",
    enable_sshd: "enableSSH", configure_firewall: "configureFirewall", install_tailscale: "installTailscale",
    authenticate_tailscale: "authenticateTailscale"
  };
  const key = keys[action.operation];
  return key ? t(key) : t("manualAction");
}

function humanReason(action: PlanAction, state: ViewState, t: Translate): string {
  if (action.operation === "configure_firewall") return `${t("safeNetworkOnly")} · ${t("port", { port: state.profile.ssh.port })}`;
  const keys: Record<string, MessageKey> = {
    install_ssh: "reasonInstallSSH",
    configure_sshd: "reasonConfigureSSH",
    configure_keys: "reasonConfigureKeys",
    enable_sshd: "reasonEnableSSH",
    install_tailscale: "reasonInstallTailscale",
    authenticate_tailscale: "reasonAuthTailscale"
  };
  const key = keys[action.operation];
  return key ? t(key) : "";
}

function humanActionIcon(action: PlanAction): string {
  if (action.layer === "firewall") return shieldIcon();
  if (action.layer === "authentication") return keyIcon();
  if (action.layer === "transport") return networkIcon();
  if (action.layer === "ssh-service") return powerIcon();
  return packageIcon();
}

function renderFriendlyProgress(state: ViewState, t: Translate): string {
  if (!state.progress.length) return `<p class="progress-note">${state.installState === "waiting-for-permission" ? t("waitingUACBody") : t("installingBody")}</p>`;
  return `<ol class="friendly-progress">${state.progress.map((event) => `<li class="${event.kind}"><span>${event.kind === "completed" ? checkIcon() : `<i></i>`}</span><div><b>${event.actionId ? humanActionLabel({ operation: operationFromID(event.actionId) } as PlanAction, t) : t("installing")}</b><p>${escapeHtml(simpleEvent(state.language, event))}</p></div></li>`).join("")}</ol>`;
}

function operationFromID(id: string): string {
  return ({ "install-ssh": "install_ssh", "configure-sshd": "configure_sshd", "configure-authorized-keys": "configure_keys", "enable-sshd": "enable_sshd", "configure-firewall": "configure_firewall", "install-tailscale": "install_tailscale", "authenticate-tailscale": "authenticate_tailscale" } as Record<string, string>)[id] ?? id;
}

export function simpleEvent(language: Language, event: { kind: string; message: string; actionId?: string }): string {
  if (event.kind === "started") return language === "zh-CN" ? "正在处理" : "In progress";
  if (event.kind === "completed") return language === "zh-CN" ? "已完成" : "Completed";
  if (event.kind === "error") return language === "zh-CN" ? "没有完成，已停止后续步骤" : "Did not finish; later steps stopped";
  return event.message;
}

function checkIssues(snapshot: Snapshot, profile: Profile, t: Translate): string[] {
  const issues: string[] = [];
  if (!snapshot.sshServer.installed) issues.push(t("issueInstall"));
  if (
    snapshot.sshPort !== profile.ssh.port || snapshot.sshPort === 0 || !snapshot.sshConfigValid
    || !snapshot.sshAuthenticationChecked || snapshot.sshPasswordAuthentication !== profile.ssh.passwordAuthentication
    || snapshot.sshKbdInteractiveAuthentication || !snapshot.sshPubkeyAuthentication
  ) issues.push(t("issueConfig"));
  if (!(snapshot.authorizedKeysChecked && (profile.ssh.passwordAuthentication || snapshot.authorizedKeysCount > 0))) issues.push(t("issueKey"));
  if (!snapshot.sshService.running || snapshot.sshService.startPolicy === "disabled") issues.push(t("issueService"));
  if (!(snapshot.firewall.checked && snapshot.firewall.enabled && !snapshot.firewall.broadExposure && (snapshot.firewall.ports ?? []).includes(profile.ssh.port))) issues.push(t("issueFirewall"));
  if (profile.transport.mode === "tailnet" && !snapshot.tailscale.online) issues.push(t("issueNetwork"));
  return issues;
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
