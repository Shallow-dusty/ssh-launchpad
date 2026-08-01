import type { Language, MessageKey } from "./i18n";
import { escapeHtml } from "./browser-utils";
import {
  arrowIcon, backIcon, checkIcon, closeIcon, computerIcon, copyIcon, devicesIcon,
  doorIcon, downloadIcon, infoIcon, keyIcon, lockIcon, monitorCheckIcon, networkIcon,
  packageIcon, powerIcon, repairIcon, screenIcon, searchIcon, shieldIcon, slidersIcon,
  uploadIcon, userIcon, warningIcon
} from "./icons";
import type { ElevatedJob, PlanAction, Profile, PublicKeyInfo, Report, Snapshot } from "./types";
import { APP_VERSION } from "./version";

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
      <footer class="privacy-foot"><span>${lockIcon()}</span>${t("noTelemetry")} · ${t("unsignedNotice")}</footer>
    </div>`;
}

function taskCard(id: string, title: string, body: string, icon: string, recommended: boolean, t: Translate): string {
  return `<button class="task-card material" data-task="${id}">${recommended ? `<span class="recommended">${t("recommended")}</span>` : ""}<span class="task-icon">${icon}</span><strong>${title}</strong><p>${body}</p><span class="task-arrow">${arrowIcon()}</span></button>`;
}

export function renderWizard(state: ViewState, t: Translate): string {
  return `
    <div class="wizard-header">
      <button class="text-button" id="wizard-back">${backIcon()} ${t("backHome")}</button>
      <div><p class="eyebrow">${state.mode === "setup" ? t("wizardSetup") : t("wizardRepair")}</p><h1>${wizardTitle(state, t)}</h1></div>
    </div>
    ${renderStepper(state, t)}
    <div class="wizard-content">
      ${state.step === 0 ? renderCheckStep(state, t) : ""}
      ${state.step === 1 ? renderRecommendationStep(state, t) : ""}
      ${state.step === 2 ? renderInstallStep(state, t) : ""}
      ${state.step === 3 ? renderTestStep(state, t) : ""}
    </div>`;
}

function wizardTitle(state: ViewState, t: Translate): string {
  return [t("stepCheck"), t("stepRecommend"), t("stepInstall"), t("stepTest")][state.step] ?? t("stepCheck");
}

function renderStepper(state: ViewState, t: Translate): string {
  return `<ol class="stepper" aria-label="${state.mode === "setup" ? t("wizardSetup") : t("wizardRepair")}">${[t("stepCheck"), t("stepRecommend"), t("stepInstall"), t("stepTest")].map((label, index) => `<li class="${index === state.step ? "active" : ""} ${index < state.step ? "done" : ""}" aria-current="${index === state.step ? "step" : "false"}"><span>${index < state.step ? checkIcon() : index + 1}</span><b>${label}</b></li>`).join("")}</ol>`;
}

function renderCheckStep(state: ViewState, t: Translate): string {
  if (state.busy && !state.report) return `<article class="focus-card material loading-card"><span class="spinner" aria-hidden="true"></span><h2>${t("checking")}</h2><p>${t("checkBody")}</p></article>`;
  const snapshot = state.report?.snapshot;
  if (!snapshot) return `<article class="focus-card material"><div class="large-symbol">${searchIcon()}</div><h2>${t("stepCheck")}</h2><p>${t("checkBody")}</p><button id="run-check" class="button primary">${t("checkNow")}</button></article>`;
  const missing = missingCount(snapshot, state.profile);
  const ready = missing === 0;
  return `
    ${resultBanner(ready ? "good" : "warn", ready ? t("ready") : t("missingSteps", { count: missing }), ready ? t("alreadyConfigured") : t("checkBody"), t)}
    <div class="plain-grid">
      ${plainCard(t("computer"), snapshot.hostname, `${snapshot.platform} · ${snapshot.arch}`, computerIcon())}
      ${plainCard(t("permission"), snapshot.isAdministrator ? t("administrator") : t("standardUser"), "", userIcon())}
      ${plainCard(t("secureNetwork"), snapshot.tailscale.online ? t("online") : t("unavailable"), snapshot.tailscale.ip ?? "", networkIcon())}
      ${plainCard(t("sshService"), snapshot.sshService.running ? t("running") : t("notRunning"), snapshot.sshService.name ?? "", powerIcon())}
    </div>
    ${technicalDetails(state.report, t)}
    <div class="wizard-actions"><button id="run-check" class="button secondary">${t("checkNow")}</button><button id="check-continue" class="button primary">${t("continue")}</button></div>`;
}

function renderRecommendationStep(state: ViewState, t: Translate): string {
  const snapshot = state.report?.snapshot;
  const tailscaleNote = !snapshot?.tailscale.installed ? t("tailscaleMissing") : !snapshot.tailscale.online ? t("tailscaleOffline") : t("recommendationBody");
  const selected = state.selectedKey?.publicKey ?? state.profile.ssh.publicKeys[0] ?? "";
  return `
    <article class="recommendation-card material">
      <div class="recommend-icon">${shieldIcon()}</div>
      <div><span class="recommended">${t("recommended")}</span><h2>${t("recommendationTitle")}</h2><p>${t("recommendationBody")}</p><aside class="info-note">${infoIcon()}<span>${tailscaleNote}</span></aside></div>
    </article>
    ${state.profile.labels.cardDisplayName ? `<aside class="card-loaded-note material">${keyIcon()}<div><strong>${t("cardLoaded", { name: state.profile.labels.cardDisplayName })}</strong>${state.profile.labels.cardNote ? `<p>${escapeHtml(state.profile.labels.cardNote)}</p>` : ""}<small>${state.profile.transport.authKey ? t("cardAuthKeyLoaded") : t("cardIncludesSettings")}</small></div></aside>` : ""}
    <article class="key-card material">
      <div class="section-title"><div><p class="eyebrow">${t("roleController")}</p><h2>${t("keyTitle")}</h2><p>${t("keyExplain")}</p></div>${lockIcon()}</div>
      ${state.detectedKeys.length ? `<div class="detected-keys"><strong>${t("foundKeys")}</strong><p>${t("foundKeysWarn")}</p>${state.detectedKeys.map((key, index) => `<label class="key-option"><input type="radio" name="controller-key" value="${index}" ${key.publicKey === selected ? "checked" : ""}/><span><b>${escapeHtml(key.label)}</b><small>${escapeHtml(fingerprintPreview(key.publicKey))}</small></span></label>`).join("")}</div>` : ""}
      <label class="field"><span>${t("pasteKey")}</span><textarea id="public-key" rows="3" placeholder="${t("pastePlaceholder")}">${escapeHtml(selected)}</textarea></label>
      <div class="key-actions"><button id="import-key" class="button secondary">${t("importKey")}</button><button id="generate-key" class="button secondary">${t("generateKey")}</button>${selected ? `<button id="export-pairing" class="button ghost">${t("exportPairing")}</button>` : ""}</div>
      <p class="small-note">${t("generateExplain")}</p>
      ${selected ? `<p class="success-note">${checkIcon()} ${t("keySelected")}</p>` : ""}
      <p id="key-error" class="inline-error"></p>
    </article>
    <div class="wizard-actions"><button id="recommend-back" class="button secondary">${backIcon()} ${t("stepCheck")}</button><button id="use-lan" class="button secondary">${state.language === "zh-CN" ? "仅在局域网使用" : "Use on LAN only"}</button><button id="use-recommended" class="button primary">${t("useRecommended")}</button></div>`;
}

function renderInstallStep(state: ViewState, t: Translate): string {
  if (state.installState === "waiting-for-permission" || state.installState === "running") {
    const waiting = state.installState === "waiting-for-permission";
    return `<article class="focus-card material loading-card"><span class="spinner" aria-hidden="true"></span><h2>${waiting ? t("waitingUAC") : t("installing")}</h2><p>${waiting ? t("waitingUACBody") : t("installingBody")}</p>${renderFriendlyProgress(state, t)}</article>`;
  }
  const plan = state.planReport?.plan;
  if (!plan) return `<article class="focus-card material loading-card"><span class="spinner"></span><h2>${t("planLoading")}</h2><p>${t("planLead")}</p></article>`;
  if (state.installState === "cancelled") return `${resultBanner("warn", t("noChanges"), t("cancelledUAC"), t)}${renderPlanBody(plan, true, state, t)}`;
  if (state.installState === "failed") return `${resultBanner("bad", t("installFailed"), t("installFailedBody"), t)}<article class="panel material"><p>${escapeHtml(state.installError || t("errorGeneric"))}</p>${technicalDetails(state.activeJob?.report, t)}</article>${renderPlanBody(plan, true, state, t)}`;
  if (plan.noChanges) return `${resultBanner("good", t("ready"), t("alreadyConfigured"), t)}<article class="focus-card material"><div class="large-symbol">${checkIcon()}</div><button id="test-now" class="button primary">${t("stepTest")}</button></article>`;
  return renderPlanBody(plan, false, state, t);
}

function renderPlanBody(plan: NonNullable<Report["plan"]>, retry: boolean, state: ViewState, t: Translate): string {
  const actions = plan.actions;
  const blockers = plan.blockers ?? [];
  const installs = actions.filter((action) => action.operation === "install_ssh" || action.operation === "install_tailscale").map((action) => humanActionLabel(action, t));
  const opensFirewall = actions.some((action) => action.operation === "configure_firewall");
  return `
    ${blockers.length ? resultBanner("bad", state.language === "zh-CN" ? "暂时不能安全继续" : "Cannot safely continue yet", blockers.join(" "), t) : ""}
    <article class="panel material">
      <div class="section-title"><div><p class="eyebrow">${t("simpleSummary")}</p><h2>${t("planTitle")}</h2><p>${t("planLead")}</p></div><span class="count-pill">${t("actionCount", { count: actions.length })}</span></div>
      <div class="human-actions">${actions.map((action) => humanAction(action, state, t)).join("")}</div>
      <div class="access-summary">
        <div><span>${packageIcon()}</span><small>${t("willInstall")}</small><b>${installs.length ? installs.join(" / ") : t("noChanges")}</b></div>
        <div><span>${doorIcon()}</span><small>${t("willOpen")}</small><b>${opensFirewall ? t("port", { port: state.profile.ssh.port }) : (state.language === "zh-CN" ? "本轮不开放" : "Not opened in this phase")}</b></div>
        <div><span>${userIcon()}</span><small>${t("whoCanConnect")}</small><b>${t("selectedController")}</b></div>
      </div>
      ${actions.some((action) => action.selfCutRisk) ? `<aside class="danger-note">${warningIcon()}<span>${state.language === "zh-CN" ? "当前操作可能切断正在使用的远程连接。请到被连接电脑本地执行，或准备第二条连接后再继续。" : "This may interrupt the current remote connection. Run locally on the target or prepare a second path."}</span></aside>` : ""}
      <details><summary>${t("technicalDetails")}</summary><pre>${escapeHtml(JSON.stringify(actions, null, 2))}</pre></details>
    </article>
    <div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("stepRecommend")}</button>${blockers.length ? "" : `<button id="open-install" class="button primary">${retry ? t("retry") : t("safeInstall")}</button>`}</div>`;
}

function renderTestStep(state: ViewState, t: Translate): string {
  if (state.busy) return `<article class="focus-card material loading-card"><span class="spinner"></span><h2>${t("testing")}</h2><p>${t("localVsRemote")}</p></article>`;
  const report = state.verifyReport ?? state.report;
  const snapshot = report?.snapshot;
  const remaining = (report?.plan?.actions.length ?? 0) + (report?.plan?.blockers?.length ?? 0) || (report?.success ? 0 : 1);
  const ready = Boolean(report?.success && remaining === 0);
  const address = state.profile.transport.mode === "tailnet" ? snapshot?.tailscale.ip : snapshot?.network.lanIps?.[0];
  const host = address || snapshot?.hostname || "HOST";
  const user = snapshot?.targetUser || (state.language === "zh-CN" ? "你的用户名" : "YOUR_USER");
  const command = `ssh -p ${state.profile.ssh.port} ${user}@${host}`;
  return `
    ${resultBanner(ready ? "good" : "warn", ready ? t("testReady") : t("testNeeds", { count: remaining }), ready ? t("testReadyBody") : t("testNeedsBody"), t)}
    <article class="connection-card material">
      <div class="connection-visual ${ready ? "ready" : ""}">${ready ? monitorCheckIcon() : devicesIcon()}<b>${ready ? t("visualReady") : t("visualWaiting")}</b></div>
      <h2>${t("connectFromOther")}</h2>
      <div class="connection-facts"><div><span>${t("host")}</span><strong>${escapeHtml(snapshot?.hostname ?? "—")}</strong></div><div><span>${t("address")}</span><strong>${escapeHtml(address ?? "—")}</strong></div><div><span>${t("portLabel")}</span><strong>${state.profile.ssh.port}</strong></div></div>
      <div class="copy-box"><code>${escapeHtml(command)}</code><button id="copy-command" class="button secondary">${copyIcon()} ${t("copyCommand")}</button></div>
      <aside class="info-note">${infoIcon()}<span>${t("nextDevice")}</span></aside>
      ${(report?.plan?.blockers?.length ?? 0) > 0 ? `<aside class="danger-note">${warningIcon()}<span>${escapeHtml(report!.plan!.blockers!.join(" "))}</span></aside>` : ""}
      <p class="boundary-note">${t("localVsRemote")}</p>
      ${technicalDetails(report, t)}
    </article>
    <div class="wizard-actions"><button id="verify-again" class="button secondary">${t("checkNow")}</button><button id="finish" class="button primary">${t("startOver")}</button></div>`;
}

export function renderAdvanced(state: ViewState, t: Translate): string {
  const profile = state.profile;
  return `
    <div class="wizard-header"><button class="text-button" id="advanced-back">${backIcon()} ${t("backHome")}</button><div><p class="eyebrow">${t("taskAdvanced")}</p><h1>${t("advancedTitle")}</h1><p>${t("advancedLead")}</p></div></div>
    <article class="panel material personal-card-panel">
      <div class="section-title"><div><p class="eyebrow">${t("personalCardTitle")}</p><h2>${t("personalCardTitle")}</h2><p>${t("personalCardLead")}</p></div>${keyIcon()}</div>
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
      <div class="panel-footer"><span id="advanced-status">${t("noTelemetry")}</span><button id="save-advanced" class="button primary">${t("saveAdvanced")}</button></div>
      ${state.report ? technicalDetails(state.report, t) : ""}
    </article>
    <article class="panel material recovery-panel">
      <div><p class="eyebrow">${t("recoveryTitle")}</p><h2>${t("recoveryTitle")}</h2><p>${t("uninstallChoice")}</p></div>
      <div class="toolbar"><button id="rollback-last" class="button secondary" ${state.report?.journalPath ? "" : "disabled"}>${t("rollbackLast")}</button><button class="button secondary" disabled title="${state.language === "zh-CN" ? `需要先有由 v${APP_VERSION} 创建的管理记录` : `Requires a v${APP_VERSION} managed-state record`}">${t("stopManaged")}</button><button id="export-report-advanced" class="button secondary">${t("exportReport")}</button><button id="check-update" class="button secondary">${t("updateCheck")}</button></div>
      <p class="small-note">${t("reportPrivacy")}</p>
    </article>
    <aside class="unsigned-banner">${warningIcon()}<span>${t("unsignedNotice")}</span></aside>`;
}

export function renderConfirmActions(state: ViewState, t: Translate): string {
  const actions = state.planReport?.plan?.actions ?? [];
  return actions.map((action) => `<div><span>${humanActionIcon(action)}</span><div><strong>${humanActionLabel(action, t)}</strong><p>${humanReason(action, state, t)}</p></div></div>`).join("");
}

function resultBanner(kind: "good" | "warn" | "bad", title: string, body: string, t: Translate): string {
  const icon = kind === "good" ? checkIcon() : kind === "warn" ? warningIcon() : closeIcon();
  return `<section class="result-banner ${kind}" role="status"><span>${icon}</span><div><p>${t("simpleSummary")}</p><h2>${escapeHtml(title)}</h2><p>${escapeHtml(body)}</p></div></section>`;
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
  if (action.operation === "configure_keys") return t("keySelected");
  return state.language === "zh-CN" ? "当前状态与推荐设置不同，需要完成这一项。" : "The current state differs from the recommended setup.";
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

function missingCount(snapshot: Snapshot, profile: Profile): number {
  const requirements = [
    snapshot.sshServer.installed,
    snapshot.sshService.running,
    snapshot.sshConfigValid,
    snapshot.sshAuthenticationChecked,
    snapshot.sshPubkeyAuthentication,
    snapshot.sshPasswordAuthentication === profile.ssh.passwordAuthentication,
    !snapshot.sshKbdInteractiveAuthentication,
    snapshot.sshAuthorizedKeysFileChecked,
    snapshot.authorizedKeysChecked && (profile.ssh.passwordAuthentication || snapshot.authorizedKeysCount > 0),
    profile.transport.mode !== "tailnet" || snapshot.tailscale.online,
    snapshot.firewall.checked && snapshot.firewall.enabled && !snapshot.firewall.broadExposure && snapshot.firewall.ports?.includes(profile.ssh.port)
  ];
  return requirements.filter((value) => !value).length;
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
