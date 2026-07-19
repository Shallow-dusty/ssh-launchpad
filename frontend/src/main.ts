import "./styles.css";
import { detectLanguage, translate, type Language, type MessageKey } from "./i18n";
import type { DesktopRequest, ElevatedJob, PlanAction, Profile, PublicKeyInfo, Report, Snapshot, Stage } from "./types";

const defaultProfile: Profile = {
  schemaVersion: 1,
  name: "recommended",
  target: { platform: "auto" },
  ssh: { enabled: true, port: 22, publicKeys: [], passwordAuthentication: false },
  transport: { mode: "tailnet", install: false },
  exposure: { mode: "tailnet", customCidrs: [] },
  download: { strategy: "official", mirrorBaseUrl: "", proxyUrl: "", offlineBundle: "", cacheDir: "", retries: 3 },
  safety: { confirmHighRisk: true, preventSelfCut: true, scheduledDelaySeconds: 20, autoRollback: true },
  advanced: { windowsSshService: "sshd", linuxSshService: "auto", macosSshLabel: "com.openssh.sshd", stateDir: "" },
  labels: { experience: "guided" }
};

type View = "home" | "wizard" | "advanced";
type WizardMode = "setup" | "repair";
type InstallState = "idle" | "waiting-for-permission" | "running" | "failed" | "cancelled" | "completed";

const state: {
  language: Language;
  view: View;
  mode: WizardMode;
  step: number;
  profile: Profile;
  report?: Report;
  planReport?: Report;
  verifyReport?: Report;
  busy: boolean;
  backend: boolean;
  detectedKeys: PublicKeyInfo[];
  selectedKey?: PublicKeyInfo;
  progress: Array<{ kind: string; message: string; actionId?: string }>;
  installState: InstallState;
  installError: string;
  activeJob?: ElevatedJob;
  toast: string;
} = {
  language: detectLanguage(),
  view: "home",
  mode: "setup",
  step: 0,
  profile: structuredClone(defaultProfile),
  busy: false,
  backend: Boolean(window.go?.main?.App),
  detectedKeys: [],
  progress: [],
  installState: "idle",
  installError: "",
  toast: ""
};

const t = (key: MessageKey, values: Record<string, string | number> = {}) => translate(state.language, key, values);

async function initialise(): Promise<void> {
  document.documentElement.lang = state.language;
  document.documentElement.dataset.theme = localStorage.getItem("ssh-launchpad-theme") ?? "";
  if (window.go?.main?.App) {
    try {
      state.profile = await window.go.main.App.DefaultProfile();
      state.profile.name = "recommended";
      state.profile.labels = { ...state.profile.labels, experience: "guided" };
      state.detectedKeys = await window.go.main.App.DiscoverPublicKeys();
    } catch (error) {
      state.toast = friendlyError(error);
    }
  } else {
    state.detectedKeys = [mockPublicKey()];
  }
  const firstKey = state.detectedKeys[0];
  if (firstKey && !state.selectedKey) {
    state.selectedKey = firstKey;
    state.profile.ssh.publicKeys = [firstKey.publicKey];
  }
  window.runtime?.EventsOn("launchpad:event", (event) => {
    state.progress.push(event);
    announce(simpleEvent(event));
    if (state.view === "wizard" && state.step === 2) renderPage();
  });
  window.runtime?.EventsOn("launchpad:second-instance", () => {
    showToast(state.language === "zh-CN"
      ? "SSH Launchpad 已经在运行；已把这个窗口带到前台。"
      : "SSH Launchpad is already running; this window was brought to the front.");
  });
  buildShell();
  renderPage();
}

function buildShell(): void {
  document.querySelector<HTMLDivElement>("#app")!.innerHTML = `
    <div class="app-shell">
      <header class="app-header material">
        <button class="brand-button" id="brand-home" aria-label="${escapeAttribute(t("backHome"))}">
          <span class="brand-mark" aria-hidden="true">${launchIcon()}</span>
          <span><strong>${t("appName")}</strong><small>${t("appTagline")}</small></span>
        </button>
        <div class="header-actions">
          <span class="backend-pill ${state.backend ? "online" : ""}">${state.backend ? t("backendNative") : t("backendDemo")}</span>
          <label class="language-select"><span class="sr-only">${t("language")}</span><select id="language" aria-label="${t("language")}"><option value="zh-CN" ${state.language === "zh-CN" ? "selected" : ""}>中文</option><option value="en" ${state.language === "en" ? "selected" : ""}>English</option></select></label>
          <button class="icon-button" id="theme-toggle" aria-label="${t("theme")}">${themeIcon()}</button>
        </div>
      </header>
      <main id="workspace" class="workspace" tabindex="-1">
        <div id="announcer" class="sr-only" aria-live="polite"></div>
        <section id="view" class="view"></section>
      </main>
      <div id="toast" class="toast ${state.toast ? "show" : ""}" role="status">${escapeHtml(state.toast)}</div>
    </div>
    <dialog id="install-dialog" aria-labelledby="install-dialog-title">
      <form method="dialog" class="dialog-card">
        <div class="dialog-symbol warning" aria-hidden="true">${shieldIcon()}</div>
        <h2 id="install-dialog-title">${t("confirmTitle")}</h2>
        <p class="muted">${t("confirmBody")}</p>
        <div id="confirm-actions" class="confirm-list"></div>
        <label class="check-row"><input id="confirm-ack" type="checkbox" /><span>${t("confirmAck")}</span></label>
        <div class="dialog-actions">
          <button value="cancel" class="button secondary">${t("cancel")}</button>
          <button id="confirm-install" value="default" class="button primary" disabled>${t("confirmInstall")}</button>
        </div>
      </form>
    </dialog>
    <input id="profile-file" class="sr-only" type="file" accept=".yaml,.yml,.json" />
    <input id="key-file" class="sr-only" type="file" accept=".pub,.txt" />
  `;
  bindGlobalEvents();
}

function bindGlobalEvents(): void {
  document.querySelector("#brand-home")?.addEventListener("click", goHome);
  document.querySelector<HTMLSelectElement>("#language")?.addEventListener("change", (event) => {
    const language = (event.currentTarget as HTMLSelectElement).value as Language;
    state.language = language;
    localStorage.setItem("ssh-launchpad-language", language);
    document.documentElement.lang = language;
    buildShell();
    renderPage();
    announce(language === "zh-CN" ? "已切换为中文" : "Switched to English");
  });
  document.querySelector("#theme-toggle")?.addEventListener("click", () => {
    const next = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    localStorage.setItem("ssh-launchpad-theme", next);
  });
  const ack = document.querySelector<HTMLInputElement>("#confirm-ack")!;
  ack.addEventListener("change", () => {
    document.querySelector<HTMLButtonElement>("#confirm-install")!.disabled = !ack.checked;
  });
  document.querySelector("#confirm-install")?.addEventListener("click", (event) => {
    event.preventDefault();
    if (!ack.checked) return;
    document.querySelector<HTMLDialogElement>("#install-dialog")!.close();
    void beginSafeInstall();
  });
  document.querySelector<HTMLInputElement>("#profile-file")?.addEventListener("change", importProfileFromBrowser);
  document.querySelector<HTMLInputElement>("#key-file")?.addEventListener("change", importKeyFromBrowser);
}

function renderPage(): void {
  const view = document.querySelector<HTMLElement>("#view")!;
  animateFromCurrent(view);
  if (state.view === "home") view.innerHTML = renderHome();
  if (state.view === "wizard") view.innerHTML = renderWizard();
  if (state.view === "advanced") view.innerHTML = renderAdvanced();
  bindPageEvents();
}

function renderHome(): string {
  return `
    <div class="home">
      <section class="welcome">
        <p class="eyebrow">${t("version")}</p>
        <h1>${t("homeTitle")}</h1>
        <p class="lead">${t("homeLead")}</p>
      </section>
      <section class="role-card material" aria-labelledby="role-title">
        <div class="role-icon" aria-hidden="true">${devicesIcon()}</div>
        <div>
          <h2 id="role-title">${t("roleTitle")}</h2>
          <div class="role-pair"><span><b>1</b>${t("roleTarget")}</span><i aria-hidden="true">${arrowIcon()}</i><span><b>2</b>${t("roleController")}</span></div>
          <p>${t("roleBody")}</p>
        </div>
      </section>
      <div class="task-grid">
        ${taskCard("setup", t("taskSetup"), t("taskSetupBody"), screenIcon(), true)}
        ${taskCard("repair", t("taskRepair"), t("taskRepairBody"), repairIcon(), false)}
        ${taskCard("advanced", t("taskAdvanced"), t("taskAdvancedBody"), slidersIcon(), false)}
      </div>
      <footer class="privacy-foot"><span>${lockIcon()}</span>${t("noTelemetry")} · ${t("unsignedNotice")}</footer>
    </div>
  `;
}

function taskCard(id: string, title: string, body: string, icon: string, recommended: boolean): string {
  return `<button class="task-card material" data-task="${id}">${recommended ? `<span class="recommended">${t("recommended")}</span>` : ""}<span class="task-icon">${icon}</span><strong>${title}</strong><p>${body}</p><span class="task-arrow">${arrowIcon()}</span></button>`;
}

function renderWizard(): string {
  return `
    <div class="wizard-header">
      <button class="text-button" id="wizard-back">${backIcon()} ${t("backHome")}</button>
      <div><p class="eyebrow">${state.mode === "setup" ? t("wizardSetup") : t("wizardRepair")}</p><h1>${wizardTitle()}</h1></div>
    </div>
    ${renderStepper()}
    <div class="wizard-content">
      ${state.step === 0 ? renderCheckStep() : ""}
      ${state.step === 1 ? renderRecommendationStep() : ""}
      ${state.step === 2 ? renderInstallStep() : ""}
      ${state.step === 3 ? renderTestStep() : ""}
    </div>
  `;
}

function wizardTitle(): string {
  return [t("stepCheck"), t("stepRecommend"), t("stepInstall"), t("stepTest")][state.step] ?? t("stepCheck");
}

function renderStepper(): string {
  return `<ol class="stepper" aria-label="${state.mode === "setup" ? t("wizardSetup") : t("wizardRepair")}">${[t("stepCheck"), t("stepRecommend"), t("stepInstall"), t("stepTest")].map((label, index) => `<li class="${index === state.step ? "active" : ""} ${index < state.step ? "done" : ""}" aria-current="${index === state.step ? "step" : "false"}"><span>${index < state.step ? checkIcon() : index + 1}</span><b>${label}</b></li>`).join("")}</ol>`;
}

function renderCheckStep(): string {
  if (state.busy && !state.report) {
    return `<article class="focus-card material loading-card"><span class="spinner" aria-hidden="true"></span><h2>${t("checking")}</h2><p>${t("checkBody")}</p></article>`;
  }
  const snapshot = state.report?.snapshot;
  if (!snapshot) {
    return `<article class="focus-card material"><div class="large-symbol">${searchIcon()}</div><h2>${t("stepCheck")}</h2><p>${t("checkBody")}</p><button id="run-check" class="button primary">${t("checkNow")}</button></article>`;
  }
  const missing = missingCount(snapshot);
  const ready = missing === 0;
  return `
    ${resultBanner(ready ? "good" : "warn", ready ? t("ready") : t("missingSteps", { count: missing }), ready ? t("alreadyConfigured") : t("checkBody"))}
    <div class="plain-grid">
      ${plainCard(t("computer"), snapshot.hostname, `${snapshot.platform} · ${snapshot.arch}`, computerIcon())}
      ${plainCard(t("permission"), snapshot.isAdministrator ? t("administrator") : t("standardUser"), "", userIcon())}
      ${plainCard(t("secureNetwork"), snapshot.tailscale.online ? t("online") : t("unavailable"), snapshot.tailscale.ip ?? "", networkIcon())}
      ${plainCard(t("sshService"), snapshot.sshService.running ? t("running") : t("notRunning"), snapshot.sshService.name ?? "", powerIcon())}
    </div>
    ${technicalDetails(state.report)}
    <div class="wizard-actions"><button id="run-check" class="button secondary">${t("checkNow")}</button><button id="check-continue" class="button primary">${t("continue")}</button></div>
  `;
}

function renderRecommendationStep(): string {
  const snapshot = state.report?.snapshot;
  const tailscaleNote = !snapshot?.tailscale.installed ? t("tailscaleMissing") : !snapshot.tailscale.online ? t("tailscaleOffline") : t("recommendationBody");
  const selected = state.selectedKey?.publicKey ?? state.profile.ssh.publicKeys[0] ?? "";
  return `
    <article class="recommendation-card material">
      <div class="recommend-icon">${shieldIcon()}</div>
      <div><span class="recommended">${t("recommended")}</span><h2>${t("recommendationTitle")}</h2><p>${t("recommendationBody")}</p><aside class="info-note">${infoIcon()}<span>${tailscaleNote}</span></aside></div>
    </article>
    <article class="key-card material">
      <div class="section-title"><div><p class="eyebrow">${t("roleController")}</p><h2>${t("keyTitle")}</h2><p>${t("keyExplain")}</p></div>${lockIcon()}</div>
      ${state.detectedKeys.length ? `<div class="detected-keys"><strong>${t("foundKeys")}</strong><p>${t("foundKeysWarn")}</p>${state.detectedKeys.map((key, index) => `<label class="key-option"><input type="radio" name="controller-key" value="${index}" ${key.publicKey === selected ? "checked" : ""}/><span><b>${escapeHtml(key.label)}</b><small>${escapeHtml(fingerprintPreview(key.publicKey))}</small></span></label>`).join("")}</div>` : ""}
      <label class="field"><span>${t("pasteKey")}</span><textarea id="public-key" rows="3" placeholder="${t("pastePlaceholder")}">${escapeHtml(selected)}</textarea></label>
      <div class="key-actions"><button id="import-key" class="button secondary">${t("importKey")}</button><button id="generate-key" class="button secondary">${t("generateKey")}</button>${selected ? `<button id="export-pairing" class="button ghost">${t("exportPairing")}</button>` : ""}</div>
      <p class="small-note">${t("generateExplain")}</p>
      ${selected ? `<p class="success-note">${checkIcon()} ${t("keySelected")}</p>` : ""}
      <p id="key-error" class="inline-error"></p>
    </article>
    <div class="wizard-actions"><button id="recommend-back" class="button secondary">${backIcon()} ${t("stepCheck")}</button><button id="use-recommended" class="button primary">${t("useRecommended")}</button></div>
  `;
}

function renderInstallStep(): string {
  if (state.installState === "waiting-for-permission" || state.installState === "running") {
    const waiting = state.installState === "waiting-for-permission";
    return `
      <article class="focus-card material loading-card"><span class="spinner" aria-hidden="true"></span><h2>${waiting ? t("waitingUAC") : t("installing")}</h2><p>${waiting ? t("waitingUACBody") : t("installingBody")}</p>${renderFriendlyProgress()}</article>
    `;
  }
  const plan = state.planReport?.plan;
  if (!plan) {
    return `<article class="focus-card material loading-card"><span class="spinner"></span><h2>${t("planLoading")}</h2><p>${t("planLead")}</p></article>`;
  }
  if (state.installState === "cancelled") {
    return `${resultBanner("warn", t("noChanges"), t("cancelledUAC"))}${renderPlanBody(plan.actions, true)}`;
  }
  if (state.installState === "failed") {
    return `${resultBanner("bad", t("installFailed"), t("installFailedBody"))}<article class="panel material"><p>${escapeHtml(state.installError || t("errorGeneric"))}</p>${technicalDetails(state.activeJob?.report)}</article>${renderPlanBody(plan.actions, true)}`;
  }
  if (plan.noChanges) {
    return `${resultBanner("good", t("ready"), t("alreadyConfigured"))}<article class="focus-card material"><div class="large-symbol">${checkIcon()}</div><button id="test-now" class="button primary">${t("stepTest")}</button></article>`;
  }
  return renderPlanBody(plan.actions, false);
}

function renderPlanBody(actions: PlanAction[], retry: boolean): string {
  return `
    <article class="panel material">
      <div class="section-title"><div><p class="eyebrow">${t("simpleSummary")}</p><h2>${t("planTitle")}</h2><p>${t("planLead")}</p></div><span class="count-pill">${t("actionCount", { count: actions.length })}</span></div>
      <div class="human-actions">${actions.map(humanAction).join("")}</div>
      <div class="access-summary">
        <div><span>${packageIcon()}</span><small>${t("willInstall")}</small><b>${actions.some((action) => action.operation.includes("install")) ? "OpenSSH / Tailscale" : t("noChanges")}</b></div>
        <div><span>${doorIcon()}</span><small>${t("willOpen")}</small><b>${t("port", { port: state.profile.ssh.port })}</b></div>
        <div><span>${userIcon()}</span><small>${t("whoCanConnect")}</small><b>${t("selectedController")}</b></div>
      </div>
      ${actions.some((action) => action.selfCutRisk) ? `<aside class="danger-note">${warningIcon()}<span>${state.language === "zh-CN" ? "当前操作可能切断正在使用的远程连接。请到被连接电脑本地执行，或准备第二条连接后再继续。" : "This may interrupt the current remote connection. Run locally on the target or prepare a second path."}</span></aside>` : ""}
      <details><summary>${t("technicalDetails")}</summary><pre>${escapeHtml(JSON.stringify(actions, null, 2))}</pre></details>
    </article>
    <div class="wizard-actions"><button id="plan-back" class="button secondary">${backIcon()} ${t("stepRecommend")}</button><button id="open-install" class="button primary">${retry ? t("retry") : t("safeInstall")}</button></div>
  `;
}

function renderTestStep(): string {
  if (state.busy) {
    return `<article class="focus-card material loading-card"><span class="spinner"></span><h2>${t("testing")}</h2><p>${t("localVsRemote")}</p></article>`;
  }
  const report = state.verifyReport ?? state.report;
  const snapshot = report?.snapshot;
  const remaining = report?.plan?.actions.length ?? (report?.success ? 0 : 1);
  const ready = Boolean(report?.success && remaining === 0);
  const host = snapshot?.tailscale.ip || snapshot?.hostname || "HOST";
  const user = state.language === "zh-CN" ? "你的用户名" : "YOUR_USER";
  const command = `ssh -p ${state.profile.ssh.port} ${user}@${host}`;
  return `
    ${resultBanner(ready ? "good" : "warn", ready ? t("testReady") : t("testNeeds", { count: remaining }), ready ? t("testReadyBody") : t("testNeedsBody"))}
    <article class="connection-card material">
      <div class="connection-visual">${devicesIcon()}<span></span>${checkIcon()}</div>
      <h2>${t("connectFromOther")}</h2>
      <div class="connection-facts"><span><small>${t("host")}</small><b>${escapeHtml(snapshot?.hostname ?? "—")}</b></span><span><small>${t("address")}</small><b>${escapeHtml(snapshot?.tailscale.ip ?? "—")}</b></span><span><small>${t("willOpen")}</small><b>${state.profile.ssh.port}</b></span></div>
      <div class="copy-box"><code>${escapeHtml(command)}</code><button id="copy-command" class="button secondary">${copyIcon()} ${t("copyCommand")}</button></div>
      <aside class="info-note">${infoIcon()}<span>${t("nextDevice")}</span></aside>
      <p class="boundary-note">${t("localVsRemote")}</p>
      ${technicalDetails(report)}
    </article>
    <div class="wizard-actions"><button id="verify-again" class="button secondary">${t("checkNow")}</button><button id="finish" class="button primary">${t("startOver")}</button></div>
  `;
}

function renderAdvanced(): string {
  const p = state.profile;
  return `
    <div class="wizard-header"><button class="text-button" id="advanced-back">${backIcon()} ${t("backHome")}</button><div><p class="eyebrow">${t("taskAdvanced")}</p><h1>${t("advancedTitle")}</h1><p>${t("advancedLead")}</p></div></div>
    <article class="panel material">
      <div class="toolbar"><button id="import-profile" class="button secondary">${uploadIcon()} ${t("importProfile")}</button><button id="export-profile" class="button secondary">${downloadIcon()} ${t("exportProfile")}</button><button id="advanced-check" class="button secondary">${t("runCheck")}</button><button id="advanced-plan" class="button primary">${t("buildPlan")}</button></div>
      <div class="form-grid">
        ${selectField("target-platform", t("targetPlatform"), p.target.platform, [["auto", "Auto"], ["windows", "Windows"], ["linux", "Linux"], ["macos", "macOS"], ["wsl", "WSL"]])}
        ${numberField("ssh-port", t("sshPort"), p.ssh.port, 1, 65535)}
        ${selectField("transport-mode", t("transport"), p.transport.mode, [["tailnet", "Tailscale"], ["lan", "LAN"], ["custom", "Custom"], ["none", "None"]])}
        ${selectField("exposure-mode", t("exposure"), p.exposure.mode, [["tailnet", "Tailnet only"], ["lan", "LAN"], ["custom", "Custom"], ["none", "None"]])}
        ${selectField("download-strategy", t("downloadSource"), p.download.strategy, [["official", "Official"], ["package-manager", "Package manager"], ["mirror", "Mirror"], ["proxy", "Proxy"], ["offline", "Offline"], ["cache", "Cache"]])}
      </div>
      <label class="field"><span>${t("publicKeys")}</span><textarea id="advanced-keys" rows="5">${escapeHtml(p.ssh.publicKeys.join("\n"))}</textarea></label>
      <label class="check-row"><input id="prevent-self-cut" type="checkbox" ${p.safety.preventSelfCut ? "checked" : ""}/><span>${state.language === "zh-CN" ? "阻止可能切断当前远程连接的操作" : "Block changes that may cut the current remote connection"}</span></label>
      <label class="check-row"><input id="auto-rollback" type="checkbox" ${p.safety.autoRollback ? "checked" : ""}/><span>${state.language === "zh-CN" ? "失败时自动恢复已完成的可恢复步骤" : "Automatically roll back completed reversible steps after failure"}</span></label>
      <div class="panel-footer"><span id="advanced-status">${t("noTelemetry")}</span><button id="save-advanced" class="button primary">${t("saveAdvanced")}</button></div>
      ${state.report ? technicalDetails(state.report) : ""}
    </article>
    <article class="panel material recovery-panel">
      <div><p class="eyebrow">${t("recoveryTitle")}</p><h2>${t("recoveryTitle")}</h2><p>${t("uninstallChoice")}</p></div>
      <div class="toolbar"><button id="rollback-last" class="button secondary" ${state.report?.journalPath ? "" : "disabled"}>${t("rollbackLast")}</button><button class="button secondary" disabled title="${state.language === "zh-CN" ? "需要先有由 v0.2.0 创建的管理记录" : "Requires a v0.2.0 managed-state record"}">${t("stopManaged")}</button><button id="export-report-advanced" class="button secondary">${t("exportReport")}</button><button id="check-update" class="button secondary">${t("updateCheck")}</button></div>
      <p class="small-note">${t("reportPrivacy")}</p>
    </article>
    <aside class="unsigned-banner">${warningIcon()}<span>${t("unsignedNotice")}</span></aside>
  `;
}

function bindPageEvents(): void {
  document.querySelectorAll<HTMLElement>("[data-task]").forEach((button) => button.addEventListener("click", () => {
    const task = button.dataset.task;
    if (task === "advanced") {
      state.view = "advanced";
      renderPage();
      return;
    }
    startWizard(task === "repair" ? "repair" : "setup");
  }));
  document.querySelector("#wizard-back")?.addEventListener("click", goHome);
  document.querySelector("#advanced-back")?.addEventListener("click", goHome);
  document.querySelector("#run-check")?.addEventListener("click", () => void runCheck());
  document.querySelector("#check-continue")?.addEventListener("click", () => { state.step = 1; renderPage(); });
  document.querySelector("#recommend-back")?.addEventListener("click", () => { state.step = 0; renderPage(); });
  document.querySelectorAll<HTMLInputElement>('input[name="controller-key"]').forEach((input) => input.addEventListener("change", () => {
    state.selectedKey = state.detectedKeys[Number(input.value)];
    if (state.selectedKey) state.profile.ssh.publicKeys = [state.selectedKey.publicKey];
    renderPage();
  }));
  document.querySelector<HTMLTextAreaElement>("#public-key")?.addEventListener("input", (event) => {
    const value = (event.currentTarget as HTMLTextAreaElement).value.trim();
    if (value) {
      state.selectedKey = { label: t("pasteKey"), path: "", publicKey: value, generated: false };
      state.profile.ssh.publicKeys = [value];
    }
  });
  document.querySelector("#import-key")?.addEventListener("click", () => void importPublicKey());
  document.querySelector("#generate-key")?.addEventListener("click", () => void generatePublicKey());
  document.querySelector("#export-pairing")?.addEventListener("click", () => void exportPairing());
  document.querySelector("#use-recommended")?.addEventListener("click", () => void useRecommended());
  document.querySelector("#plan-back")?.addEventListener("click", () => { state.step = 1; state.installState = "idle"; renderPage(); });
  document.querySelector("#open-install")?.addEventListener("click", openInstallDialog);
  document.querySelector("#test-now")?.addEventListener("click", () => void runVerify());
  document.querySelector("#verify-again")?.addEventListener("click", () => void runVerify());
  document.querySelector("#copy-command")?.addEventListener("click", copyConnectionCommand);
  document.querySelector("#finish")?.addEventListener("click", goHome);
  document.querySelector("#import-profile")?.addEventListener("click", () => void importProfile());
  document.querySelector("#export-profile")?.addEventListener("click", () => void exportProfile());
  document.querySelector("#save-advanced")?.addEventListener("click", saveAdvanced);
  document.querySelector("#advanced-check")?.addEventListener("click", () => void runAdvancedStage("check"));
  document.querySelector("#advanced-plan")?.addEventListener("click", () => void runAdvancedStage("plan"));
  document.querySelector("#export-report-advanced")?.addEventListener("click", () => void exportReport());
  document.querySelector("#check-update")?.addEventListener("click", () => void checkForUpdate());
  document.querySelector("#rollback-last")?.addEventListener("click", () => void rollbackLast());
}

function startWizard(mode: WizardMode): void {
  state.view = "wizard";
  state.mode = mode;
  state.step = 0;
  state.report = undefined;
  state.planReport = undefined;
  state.verifyReport = undefined;
  state.progress = [];
  state.installState = "idle";
  state.installError = "";
  renderPage();
  void runCheck();
}

async function runCheck(): Promise<void> {
  if (state.busy) return;
  state.busy = true;
  renderPage();
  try {
    state.report = await runStage("check");
  } catch (error) {
    state.toast = friendlyError(error);
  } finally {
    state.busy = false;
    renderPage();
  }
}

async function useRecommended(): Promise<void> {
  const textarea = document.querySelector<HTMLTextAreaElement>("#public-key");
  const key = textarea?.value.trim() || state.selectedKey?.publicKey || state.profile.ssh.publicKeys[0];
  if (!key || key.includes("PRIVATE KEY") || !key.startsWith("ssh-")) {
    const error = document.querySelector<HTMLElement>("#key-error");
    if (error) error.textContent = t("keyRequired");
    return;
  }
  state.profile.ssh.publicKeys = [key];
  state.profile.ssh.passwordAuthentication = false;
  state.profile.transport.mode = "tailnet";
  state.profile.exposure.mode = "tailnet";
  state.profile.transport.install = !state.report?.snapshot?.tailscale.installed;
  state.step = 2;
  state.busy = true;
  renderPage();
  try {
    state.planReport = await runStage("plan");
  } catch (error) {
    state.installState = "failed";
    state.installError = friendlyError(error);
  } finally {
    state.busy = false;
    renderPage();
  }
}

function openInstallDialog(): void {
  const actions = state.planReport?.plan?.actions ?? [];
  document.querySelector("#confirm-actions")!.innerHTML = actions.map((action) => `<div><span>${humanActionIcon(action)}</span><div><strong>${humanActionLabel(action)}</strong><p>${humanReason(action)}</p></div></div>`).join("");
  const ack = document.querySelector<HTMLInputElement>("#confirm-ack")!;
  ack.checked = false;
  document.querySelector<HTMLButtonElement>("#confirm-install")!.disabled = true;
  document.querySelector<HTMLDialogElement>("#install-dialog")!.showModal();
  requestAnimationFrame(() => ack.focus());
}

async function beginSafeInstall(): Promise<void> {
  state.installState = "waiting-for-permission";
  state.installError = "";
  state.progress = [];
  renderPage();
  const request: DesktopRequest = { stage: "apply", profile: state.profile, confirmed: true, allowSelfCut: false, scheduleRisky: false, externalVerify: "" };
  try {
    if (window.go?.main?.App) {
      state.activeJob = await window.go.main.App.BeginElevatedApply(request);
      await pollElevatedJob(state.activeJob.id);
    } else {
      state.activeJob = await mockElevatedApply(request);
      finishElevatedJob(state.activeJob);
    }
  } catch (error) {
    state.installState = "failed";
    state.installError = friendlyError(error);
    renderPage();
  }
}

async function pollElevatedJob(id: string): Promise<void> {
  while (true) {
    const job = await window.go!.main!.App!.ElevatedApplyStatus(id);
    state.activeJob = job;
    state.progress = job.events ?? [];
    state.installState = job.state === "waiting-for-permission" ? "waiting-for-permission" : job.state === "running" ? "running" : job.state;
    renderPage();
    if (["completed", "failed", "cancelled"].includes(job.state)) {
      finishElevatedJob(job);
      await window.go!.main!.App!.DismissElevatedJob(id);
      return;
    }
    await delay(500);
  }
}

function finishElevatedJob(job: ElevatedJob): void {
  if (job.state === "cancelled") {
    state.installState = "cancelled";
    state.installError = job.error ?? t("cancelledUAC");
    renderPage();
    return;
  }
  if (job.state === "failed" || !job.report?.success) {
    state.installState = "failed";
    state.installError = job.error ?? job.report?.error ?? t("errorGeneric");
    renderPage();
    return;
  }
  state.installState = "completed";
  state.report = job.report;
  localStorage.setItem("ssh-launchpad-demo-ready", "true");
  void runVerify();
}

async function runVerify(): Promise<void> {
  state.step = 3;
  state.busy = true;
  renderPage();
  try {
    state.verifyReport = await runStage("verify");
  } catch (error) {
    state.verifyReport = state.activeJob?.report;
    state.toast = friendlyError(error);
  } finally {
    state.busy = false;
    renderPage();
  }
}

async function runStage(stage: Stage): Promise<Report> {
  const request: DesktopRequest = { stage, profile: state.profile, confirmed: false, allowSelfCut: false, scheduleRisky: false, externalVerify: "" };
  return window.go?.main?.App ? window.go.main.App.Run(request) : mockRun(request);
}

async function runAdvancedStage(stage: "check" | "plan"): Promise<void> {
  saveAdvanced();
  state.busy = true;
  try {
    state.report = await runStage(stage);
    showToast(stage === "check" ? t("runCheck") : t("buildPlan"));
  } catch (error) {
    showToast(friendlyError(error));
  } finally {
    state.busy = false;
    renderPage();
  }
}

async function importPublicKey(): Promise<void> {
  try {
    if (window.go?.main?.App) {
      const key = await window.go.main.App.ImportPublicKey();
      if (key.publicKey) selectKey(key);
    } else {
      document.querySelector<HTMLInputElement>("#key-file")!.click();
    }
  } catch (error) {
    showToast(friendlyError(error));
  }
}

async function generatePublicKey(): Promise<void> {
  try {
    const key = window.go?.main?.App ? await window.go.main.App.GenerateControllerKey("ssh-launchpad-controller") : mockPublicKey(true);
    selectKey(key);
    showToast(t("keySelected"));
  } catch (error) {
    showToast(friendlyError(error));
  }
}

function selectKey(key: PublicKeyInfo): void {
  state.selectedKey = key;
  state.profile.ssh.publicKeys = [key.publicKey];
  if (!state.detectedKeys.some((existing) => existing.publicKey === key.publicKey)) state.detectedKeys.push(key);
  renderPage();
}

async function exportPairing(): Promise<void> {
  const key = state.selectedKey?.publicKey ?? state.profile.ssh.publicKeys[0];
  if (!key) return;
  if (window.go?.main?.App) {
    const path = await window.go.main.App.ExportPairingFile(key);
    if (path) showToast(path);
  } else {
    downloadText("ssh-launchpad-controller.pub", `${key}\n`, "text/plain;charset=utf-8");
    showToast(t("exportPairing"));
  }
}

async function importProfile(): Promise<void> {
  try {
    if (window.go?.main?.App) {
      const profile = await window.go.main.App.ImportProfile();
      if (profile.schemaVersion) {
        state.profile = profile;
        state.selectedKey = profile.ssh.publicKeys[0] ? { label: t("profileImported"), path: "", publicKey: profile.ssh.publicKeys[0], generated: false } : undefined;
        showToast(t("profileImported"));
        renderPage();
      }
    } else {
      document.querySelector<HTMLInputElement>("#profile-file")!.click();
    }
  } catch (error) {
    showToast(friendlyError(error));
  }
}

async function exportProfile(): Promise<void> {
  saveAdvanced();
  if (window.go?.main?.App) {
    const path = await window.go.main.App.ExportProfile(state.profile);
    if (path) showToast(t("profileExported"));
    return;
  }
  downloadText(`${state.profile.name}.ssh-launchpad.yaml`, profileToYAML(state.profile), "text/yaml;charset=utf-8");
  showToast(t("profileExported"));
}

function saveAdvanced(): void {
  const platform = document.querySelector<HTMLSelectElement>("#target-platform");
  if (!platform) return;
  state.profile.target.platform = platform.value;
  state.profile.ssh.port = Number(valueOf("ssh-port"));
  state.profile.transport.mode = valueOf("transport-mode");
  state.profile.exposure.mode = valueOf("exposure-mode");
  state.profile.download.strategy = valueOf("download-strategy");
  state.profile.ssh.publicKeys = valueOf("advanced-keys").split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  state.profile.safety.preventSelfCut = checked("prevent-self-cut");
  state.profile.safety.autoRollback = checked("auto-rollback");
  setText("#advanced-status", t("advancedSaved"));
  announce(t("advancedSaved"));
}

async function exportReport(): Promise<void> {
  if (!state.report) {
    showToast(t("noChanges"));
    return;
  }
  if (window.go?.main?.App) {
    const path = await window.go.main.App.ExportReport(state.report);
    if (path) showToast(path);
  } else {
    downloadText(`${state.report.id}.report.json`, `${JSON.stringify(redactReport(state.report), null, 2)}\n`, "application/json;charset=utf-8");
  }
}

async function checkForUpdate(): Promise<void> {
  try {
    const info = window.go?.main?.App
      ? await window.go.main.App.CheckForUpdate()
      : await fetch("https://api.github.com/repos/Shallow-dusty/ssh-launchpad/releases/latest")
        .then(async (response) => {
          if (!response.ok) throw new Error(`HTTP ${response.status}`);
          const value = await response.json() as { tag_name: string; html_url: string };
          const latest = value.tag_name.replace(/^v/, "");
          return { currentVersion: "0.2.0", latestVersion: latest, available: isNewerVersion(latest, "0.2.0"), url: value.html_url, channel: "stable" };
        });
    if (info.available) {
      const message = state.language === "zh-CN"
        ? `发现稳定版 ${info.latestVersion}。只打开下载页，不会静默更新系统组件。`
        : `Stable ${info.latestVersion} is available. Only the download page will open; system components are never silently updated.`;
      showToast(message);
      window.open(info.url, "_blank", "noopener,noreferrer");
    } else {
      showToast(state.language === "zh-CN" ? "当前已是最新稳定版。" : "You have the latest stable version.");
    }
  } catch (error) {
    showToast(friendlyError(error));
  }
}

function isNewerVersion(candidate: string, current: string): boolean {
  const parse = (value: string) => value.replace(/^v/, "").split(".").slice(0, 3).map((part) => Number(part.split("-")[0] ?? 0) || 0);
  const left = parse(candidate);
  const right = parse(current);
  for (let index = 0; index < 3; index++) {
    if ((left[index] ?? 0) !== (right[index] ?? 0)) return (left[index] ?? 0) > (right[index] ?? 0);
  }
  return false;
}

async function rollbackLast(): Promise<void> {
  if (!state.report?.journalPath || !window.go?.main?.App) return;
  if (!confirm(t("rollbackLast"))) return;
  try {
    const report = await window.go.main.App.Rollback(state.report.journalPath);
    state.report = report;
    showToast(report.success ? t("ready") : t("errorGeneric"));
    renderPage();
  } catch (error) {
    showToast(friendlyError(error));
  }
}

function importProfileFromBrowser(event: Event): void {
  const file = (event.currentTarget as HTMLInputElement).files?.[0];
  if (!file) return;
  void file.text().then((text) => {
    try {
      const profile = JSON.parse(text) as Profile;
      if (profile.schemaVersion !== 1) throw new Error("schemaVersion");
      state.profile = profile;
      showToast(t("profileImported"));
      renderPage();
    } catch {
      showToast(state.language === "zh-CN" ? "浏览器预览仅导入 JSON；桌面应用支持 YAML 和 JSON。" : "Browser preview imports JSON; the desktop app supports YAML and JSON.");
    }
  });
}

function importKeyFromBrowser(event: Event): void {
  const file = (event.currentTarget as HTMLInputElement).files?.[0];
  if (!file) return;
  void file.text().then((text) => {
    if (text.includes("PRIVATE KEY")) {
      showToast(state.language === "zh-CN" ? "拒绝导入私钥。请选择 .pub 公钥文件。" : "Private keys are rejected. Choose a .pub file.");
      return;
    }
    const key = text.split(/\r?\n/).map((line) => line.trim()).find((line) => line.startsWith("ssh-"));
    if (key) selectKey({ label: file.name, path: file.name, publicKey: key, generated: false });
  });
}

async function copyConnectionCommand(): Promise<void> {
  const code = document.querySelector<HTMLElement>(".copy-box code")?.textContent ?? "";
  await navigator.clipboard.writeText(code);
  showToast(t("copied"));
}

function goHome(): void {
  state.view = "home";
  state.step = 0;
  state.installState = "idle";
  state.installError = "";
  renderPage();
}

function resultBanner(kind: "good" | "warn" | "bad", title: string, body: string): string {
  const icon = kind === "good" ? checkIcon() : kind === "warn" ? warningIcon() : closeIcon();
  return `<section class="result-banner ${kind}" role="status"><span>${icon}</span><div><p>${t("simpleSummary")}</p><h2>${title}</h2><p>${body}</p></div></section>`;
}

function plainCard(label: string, value: string, note: string, icon: string): string {
  return `<article class="plain-card material"><span>${icon}</span><div><small>${label}</small><strong>${escapeHtml(value)}</strong>${note ? `<p>${escapeHtml(note)}</p>` : ""}</div></article>`;
}

function technicalDetails(report?: Report): string {
  if (!report) return "";
  return `<details class="technical-details"><summary>${t("details")}</summary><div class="detail-grid">${report.snapshot ? `<div><b>${t("system")}</b><pre>${escapeHtml(JSON.stringify(report.snapshot, null, 2))}</pre></div>` : ""}${report.plan ? `<div><b>${t("rawReport")}</b><pre>${escapeHtml(JSON.stringify(report.plan, null, 2))}</pre></div>` : ""}</div></details>`;
}

function humanAction(action: PlanAction): string {
  return `<div class="human-action"><span>${humanActionIcon(action)}</span><div><strong>${humanActionLabel(action)}</strong><p>${humanReason(action)}</p></div><b>${action.requiresElevation ? shieldIcon() : checkIcon()}</b></div>`;
}

function humanActionLabel(action: PlanAction): string {
  const keys: Record<string, MessageKey> = {
    install_ssh: "installSSH",
    configure_sshd: "configureSSH",
    configure_keys: "configureKeys",
    enable_sshd: "enableSSH",
    configure_firewall: "configureFirewall",
    install_tailscale: "installTailscale"
  };
  const key = keys[action.operation];
  return key ? t(key) : t("manualAction");
}

function humanReason(action: PlanAction): string {
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

function renderFriendlyProgress(): string {
  if (!state.progress.length) return `<p class="progress-note">${state.installState === "waiting-for-permission" ? t("waitingUACBody") : t("installingBody")}</p>`;
  return `<ol class="friendly-progress">${state.progress.map((event) => `<li class="${event.kind}"><span>${event.kind === "completed" ? checkIcon() : `<i></i>`}</span><div><b>${event.actionId ? humanActionLabel({ operation: operationFromID(event.actionId) } as PlanAction) : t("installing")}</b><p>${simpleEvent(event)}</p></div></li>`).join("")}</ol>`;
}

function operationFromID(id: string): string {
  return ({ "install-ssh": "install_ssh", "configure-sshd": "configure_sshd", "configure-authorized-keys": "configure_keys", "enable-sshd": "enable_sshd", "configure-firewall": "configure_firewall", "install-tailscale": "install_tailscale" } as Record<string, string>)[id] ?? id;
}

function simpleEvent(event: { kind: string; message: string; actionId?: string }): string {
  if (event.kind === "started") return state.language === "zh-CN" ? "正在处理" : "In progress";
  if (event.kind === "completed") return state.language === "zh-CN" ? "已完成" : "Completed";
  if (event.kind === "error") return state.language === "zh-CN" ? "没有完成，已停止后续步骤" : "Did not finish; later steps stopped";
  return event.message;
}

function missingCount(snapshot: Snapshot): number {
  return [snapshot.sshServer.installed, snapshot.sshService.running, snapshot.sshConfigValid, snapshot.tailscale.online, snapshot.firewall.ports?.includes(state.profile.ssh.port)].filter((value) => !value).length;
}

async function mockRun(request: DesktopRequest): Promise<Report> {
  await delay(180);
  const configured = localStorage.getItem("ssh-launchpad-demo-ready") === "true";
  const snapshot: Snapshot = {
    platform: "windows",
    arch: "amd64",
    hostname: "HOME-PC",
    isAdministrator: false,
    sessionTransport: "local",
    packageManager: "winget",
    sshClient: { installed: true, version: "OpenSSH_9.x" },
    sshServer: { installed: configured },
    sshService: { name: "sshd", installed: configured, running: configured, startPolicy: configured ? "Automatic" : "Manual" },
    sshPort: configured ? request.profile.ssh.port : 0,
    sshConfigValid: true,
    firewall: { provider: "windows-firewall", ports: configured ? [request.profile.ssh.port] : [], scopes: configured ? ["100.64.0.0/10", "fd7a:115c:a1e0::/48"] : [] },
    tailscale: { installed: true, online: true, ip: "100.64.10.25", state: "Running" },
    network: { githubDns: true, tailscaleDns: true, proxySet: false }
  };
  const actions = configured ? [] : mockActions(request.profile.ssh.port);
  return {
    id: `${request.stage}-${Date.now()}`,
    stage: request.stage,
    success: request.stage !== "verify" || configured,
    exitCode: request.stage === "verify" && !configured ? 3 : 0,
    profileName: request.profile.name,
    snapshot,
    plan: { noChanges: actions.length === 0, highestRisk: actions.length ? "high" : "low", selfCutDetected: false, actions }
  };
}

async function mockElevatedApply(request: DesktopRequest): Promise<ElevatedJob> {
  await delay(250);
  const mode = new URLSearchParams(location.search).get("mock");
  const attempt = Number(sessionStorage.getItem("ssh-launchpad-mock-attempt") ?? "0") + 1;
  sessionStorage.setItem("ssh-launchpad-mock-attempt", String(attempt));
  if (mode === "uac-cancel" && attempt === 1) return { id: "mock", state: "cancelled", error: t("cancelledUAC") };
  if (mode === "fail" && attempt === 1) return { id: "mock", state: "failed", error: state.language === "zh-CN" ? "模拟：网络中断，校验失败，电脑没有继续改动。" : "Simulated network interruption; verification failed and later changes stopped." };
  state.installState = "running";
  for (const action of mockActions(request.profile.ssh.port)) {
    state.progress.push({ kind: "started", actionId: action.id, message: action.summary });
    renderPage();
    await delay(140);
    state.progress.push({ kind: "completed", actionId: action.id, message: "completed" });
    renderPage();
  }
  localStorage.setItem("ssh-launchpad-demo-ready", "true");
  const report = await mockRun({ ...request, stage: "apply" });
  report.success = true;
  report.exitCode = 0;
  report.results = mockActions(request.profile.ssh.port).map((action) => ({ actionId: action.id, status: "completed" }));
  return { id: "mock", state: "completed", report };
}

function mockActions(port: number): PlanAction[] {
  return [
    { id: "install-ssh", operation: "install_ssh", layer: "ssh-packages", risk: "medium", summary: "Install OpenSSH", reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: false },
    { id: "configure-sshd", operation: "configure_sshd", layer: "ssh-config", risk: "high", summary: "Configure SSH", reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true },
    { id: "configure-authorized-keys", operation: "configure_keys", layer: "authentication", risk: "high", summary: "Add controller key", reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true },
    { id: "enable-sshd", operation: "enable_sshd", layer: "ssh-service", risk: "medium", summary: "Start service", reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true },
    { id: "configure-firewall", operation: "configure_firewall", layer: "firewall", risk: "high", summary: `Allow ${port}`, reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true }
  ];
}

function mockPublicKey(generated = false): PublicKeyInfo {
  return { label: generated ? "id_ed25519_ssh_launchpad.pub" : "id_ed25519.pub", path: "C:\\Users\\demo\\.ssh\\id_ed25519.pub", privateKeyPath: "C:\\Users\\demo\\.ssh\\id_ed25519", generated, publicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMockPublicKeyForSafeLaunchpadPreview controller" };
}

function profileToYAML(profile: Profile): string {
  const scalar = (value: unknown) => JSON.stringify(value);
  return `schemaVersion: ${profile.schemaVersion}\nname: ${scalar(profile.name)}\ntarget:\n  platform: ${profile.target.platform}\nssh:\n  enabled: ${profile.ssh.enabled}\n  port: ${profile.ssh.port}\n  publicKeys:\n${profile.ssh.publicKeys.map((key) => `    - ${scalar(key)}`).join("\n")}\n  passwordAuthentication: ${profile.ssh.passwordAuthentication}\ntransport:\n  mode: ${profile.transport.mode}\n  install: ${profile.transport.install}\nexposure:\n  mode: ${profile.exposure.mode}\n  customCidrs: []\ndownload:\n  strategy: ${profile.download.strategy}\n  mirrorBaseUrl: ${scalar(profile.download.mirrorBaseUrl)}\n  proxyUrl: ${scalar(profile.download.proxyUrl)}\n  offlineBundle: ${scalar(profile.download.offlineBundle)}\n  cacheDir: ${scalar(profile.download.cacheDir)}\n  retries: ${profile.download.retries}\nsafety:\n  confirmHighRisk: true\n  preventSelfCut: ${profile.safety.preventSelfCut}\n  scheduledDelaySeconds: ${profile.safety.scheduledDelaySeconds}\n  autoRollback: ${profile.safety.autoRollback}\nadvanced:\n  windowsSshService: sshd\n  linuxSshService: auto\n  macosSshLabel: com.openssh.sshd\n  stateDir: ${scalar(profile.advanced.stateDir)}\nlabels:\n  experience: guided\n`;
}

function redactReport(report: Report): Report {
  const clone = structuredClone(report);
  if (clone.snapshot?.tailscale.ip) clone.snapshot.tailscale.ip = "<redacted-ip>";
  if (clone.snapshot?.hostname) clone.snapshot.hostname = "<redacted-host>";
  return clone;
}

function fingerprintPreview(key: string): string {
  const parts = key.split(/\s+/);
  const data = parts[1] ?? "";
  return `${parts[0] ?? "public-key"} · ${data.slice(0, 12)}…${data.slice(-8)}`;
}

function showToast(message: string): void {
  state.toast = message;
  const toast = document.querySelector<HTMLElement>("#toast");
  if (toast) {
    toast.textContent = message;
    toast.classList.add("show");
    setTimeout(() => toast.classList.remove("show"), 2800);
  }
  announce(message);
}

function friendlyError(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error ?? "");
  if (/cancel/i.test(raw)) return t("cancelledUAC");
  if (/checksum|sha256/i.test(raw)) return state.language === "zh-CN" ? "下载文件校验失败，没有执行安装。请重试或改用已验证的离线包。" : "Download verification failed. Nothing was installed; retry or use a verified offline bundle.";
  if (/network|timeout|resolve|dns/i.test(raw)) return state.language === "zh-CN" ? "网络暂时不可用。可检查代理、改用官方镜像或离线包后重试。" : "Network unavailable. Check proxy settings, use an explicit trusted mirror, or retry with an offline bundle.";
  return raw || t("errorGeneric");
}

function selectField(id: string, label: string, value: string, options: Array<[string, string]>): string {
  return `<label class="field"><span>${label}</span><select id="${id}">${options.map(([key, text]) => `<option value="${key}" ${key === value ? "selected" : ""}>${text}</option>`).join("")}</select></label>`;
}

function numberField(id: string, label: string, value: number, min: number, max: number): string {
  return `<label class="field"><span>${label}</span><input id="${id}" type="number" value="${value}" min="${min}" max="${max}"/></label>`;
}

function valueOf(id: string): string { return document.querySelector<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>(`#${id}`)?.value ?? ""; }
function checked(id: string): boolean { return document.querySelector<HTMLInputElement>(`#${id}`)?.checked ?? false; }
function setText(selector: string, text: string): void { const node = document.querySelector<HTMLElement>(selector); if (node) node.textContent = text; }
function announce(text: string): void { setText("#announcer", text); }
function delay(ms: number): Promise<void> { return new Promise((resolve) => setTimeout(resolve, ms)); }
function escapeHtml(value: unknown): string { return String(value ?? "").replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" })[char]!); }
function escapeAttribute(value: unknown): string { return escapeHtml(value); }
function downloadText(name: string, text: string, type: string): void { const url = URL.createObjectURL(new Blob([text], { type })); const link = document.createElement("a"); link.href = url; link.download = name; link.click(); URL.revokeObjectURL(url); }
function animateFromCurrent(element: HTMLElement): void { if (matchMedia("(prefers-reduced-motion: reduce)").matches) return; element.getAnimations().forEach((animation) => animation.cancel()); element.animate([{ opacity: .65, transform: "translateY(5px)" }, { opacity: 1, transform: "translateY(0)" }], { duration: 220, easing: "cubic-bezier(.2,.8,.2,1)" }); }

function svg(content: string): string { return `<svg viewBox="0 0 24 24" aria-hidden="true">${content}</svg>`; }
function launchIcon(): string { return svg('<path d="M5 8.5 12 4l7 4.5v7L12 20l-7-4.5z"/><path d="m9 11 2 2-2 2m4 0h3"/>'); }
function themeIcon(): string { return svg('<path d="M12 3a9 9 0 1 0 9 9c-5 2-11-4-9-9z"/>'); }
function devicesIcon(): string { return svg('<rect x="2.5" y="5" width="11" height="8" rx="1.5"/><path d="M6 17h4m-2-4v4"/><rect x="15.5" y="8" width="6" height="10" rx="1.3"/>'); }
function arrowIcon(): string { return svg('<path d="M5 12h14m-5-5 5 5-5 5"/>'); }
function screenIcon(): string { return svg('<rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8m-4-4v4"/>'); }
function repairIcon(): string { return svg('<path d="M14.5 6.5a4 4 0 0 0-5-5L12 4 9 7 6.5 4.5a4 4 0 0 0 5 5L19 17a1.4 1.4 0 0 1-2 2z"/>'); }
function slidersIcon(): string { return svg('<path d="M4 7h10m4 0h2M4 17h2m4 0h10M14 4v6M6 14v6"/>'); }
function lockIcon(): string { return svg('<rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>'); }
function backIcon(): string { return svg('<path d="m15 18-6-6 6-6"/>'); }
function searchIcon(): string { return svg('<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>'); }
function checkIcon(): string { return svg('<path d="m5 12 4 4L19 6"/>'); }
function shieldIcon(): string { return svg('<path d="M12 3 20 6v5c0 5-3.4 8.2-8 10-4.6-1.8-8-5-8-10V6z"/><path d="m8.5 12 2.2 2.2 4.8-5"/>'); }
function infoIcon(): string { return svg('<circle cx="12" cy="12" r="9"/><path d="M12 11v5m0-8h.01"/>'); }
function warningIcon(): string { return svg('<path d="m12 3 10 18H2z"/><path d="M12 9v5m0 3h.01"/>'); }
function closeIcon(): string { return svg('<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6m0-6-6 6"/>'); }
function computerIcon(): string { return screenIcon(); }
function userIcon(): string { return svg('<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>'); }
function networkIcon(): string { return svg('<circle cx="12" cy="12" r="2"/><path d="M5.6 18.4a9 9 0 0 1 0-12.8m12.8 0a9 9 0 0 1 0 12.8M8.5 15.5a5 5 0 0 1 0-7m7 0a5 5 0 0 1 0 7"/>'); }
function powerIcon(): string { return svg('<path d="M12 2v10m6.4-6.4a9 9 0 1 1-12.8 0"/>'); }
function packageIcon(): string { return svg('<path d="m4 7 8-4 8 4v10l-8 4-8-4z"/><path d="m4 7 8 4 8-4m-8 4v10"/>'); }
function doorIcon(): string { return svg('<path d="M5 21V4l12-2v19M5 21h14"/><path d="M13 12h.01"/>'); }
function keyIcon(): string { return svg('<circle cx="8" cy="15" r="4"/><path d="m11 12 8-8m-3 3 3 3"/>'); }
function copyIcon(): string { return svg('<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>'); }
function uploadIcon(): string { return svg('<path d="M12 16V4m-5 5 5-5 5 5M4 20h16"/>'); }
function downloadIcon(): string { return svg('<path d="M12 4v12m-5-5 5 5 5-5M4 20h16"/>'); }

void initialise();
