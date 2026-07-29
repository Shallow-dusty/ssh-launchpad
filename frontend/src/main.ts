import "./styles.css";
import { detectLanguage, translate, type Language, type MessageKey } from "./i18n";
import {
  animateFromCurrent, announce, checked, delay, downloadText, escapeAttribute,
  escapeHtml, setText, valueOf
} from "./browser-utils";
import { launchIcon, shieldIcon, themeIcon } from "./icons";
import { mockActions, mockPublicKey, mockRun } from "./mock-backend";
import { isNewerVersion, profileToYAML, redactReport } from "./model-utils";
import {
  renderAdvanced, renderConfirmActions, renderHome, renderWizard, simpleEvent,
  type InstallState, type WizardMode
} from "./views";
import type { DesktopRequest, ElevatedJob, Profile, PublicKeyInfo, Report, Stage } from "./types";
import { APP_VERSION } from "./version";

const defaultProfile: Profile = {
  schemaVersion: 1,
  name: "recommended",
  target: { platform: "auto" },
  ssh: { enabled: true, port: 22, publicKeys: [], passwordAuthentication: false },
  transport: { mode: "tailnet", install: false },
  exposure: { mode: "tailnet", customCidrs: [] },
  download: { strategy: "official", mirrorBaseUrl: "", proxyUrl: "", offlineBundle: "", offlineSha256: "", cacheDir: "", retries: 3 },
  safety: { confirmHighRisk: true, preventSelfCut: true, scheduledDelaySeconds: 20, autoRollback: true },
  advanced: { windowsSshService: "sshd", linuxSshService: "auto", macosSshLabel: "com.openssh.sshd", stateDir: "" },
  labels: { experience: "guided" }
};

type View = "home" | "wizard" | "advanced";
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

function normalizeProfile(profile: Profile): Profile {
  const source = profile as Partial<Profile>;
  const defaults = structuredClone(defaultProfile);
  return {
    ...defaults,
    ...source,
    target: { ...defaults.target, ...source.target },
    ssh: {
      ...defaults.ssh,
      ...source.ssh,
      publicKeys: Array.isArray(source.ssh?.publicKeys) ? [...source.ssh.publicKeys] : []
    },
    transport: { ...defaults.transport, ...source.transport },
    exposure: {
      ...defaults.exposure,
      ...source.exposure,
      customCidrs: Array.isArray(source.exposure?.customCidrs) ? [...source.exposure.customCidrs] : []
    },
    download: { ...defaults.download, ...source.download },
    safety: { ...defaults.safety, ...source.safety },
    advanced: { ...defaults.advanced, ...source.advanced },
    labels: { ...defaults.labels, ...(source.labels ?? {}) }
  };
}

async function initialise(): Promise<void> {
  document.documentElement.lang = state.language;
  document.documentElement.dataset.theme = localStorage.getItem("ssh-launchpad-theme") ?? "";
  if (window.go?.main?.App) {
    try {
      state.profile = await window.go.main.App.DefaultProfile();
      state.profile.name = "recommended";
      state.profile.labels = { ...state.profile.labels, experience: "guided" };
      state.detectedKeys = await window.go.main.App.DiscoverPublicKeys() ?? [];
    } catch (error) {
      state.toast = friendlyError(error);
    }
  } else {
    const mock = new URLSearchParams(window.location.search).get("mock");
    state.detectedKeys = mock === "no-public-key" ? [] : [mockPublicKey()];
    if (mock === "no-public-key") {
      delete (state.profile.ssh as Partial<Profile["ssh"]>).publicKeys;
    }
  }
  state.profile = normalizeProfile(state.profile);
  window.runtime?.EventsOn("launchpad:event", (event) => {
    state.progress.push(event);
    announce(simpleEvent(state.language, event));
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
    state.profile = normalizeProfile(state.profile);
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
  if (state.view === "home") view.innerHTML = renderHome(state, t);
  if (state.view === "wizard") view.innerHTML = renderWizard(state, t);
  if (state.view === "advanced") view.innerHTML = renderAdvanced(state, t);
  bindPageEvents();
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
    } else {
      state.selectedKey = undefined;
      state.profile.ssh.publicKeys = [];
    }
  });
  document.querySelector("#import-key")?.addEventListener("click", () => void importPublicKey());
  document.querySelector("#generate-key")?.addEventListener("click", () => void generatePublicKey());
  document.querySelector("#export-pairing")?.addEventListener("click", () => void exportPairing());
  document.querySelector("#use-recommended")?.addEventListener("click", () => void useRecommended());
  document.querySelector("#use-lan")?.addEventListener("click", () => void useLAN());
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
  await useGuidedMode("tailnet");
}

async function useLAN(): Promise<void> {
  await useGuidedMode("lan");
}

async function useGuidedMode(mode: "tailnet" | "lan"): Promise<void> {
  const textarea = document.querySelector<HTMLTextAreaElement>("#public-key");
  const key = textarea?.value.trim() || state.selectedKey?.publicKey || state.profile.ssh.publicKeys[0];
  if (!key || !(await publicKeyIsValid(key))) {
    const error = document.querySelector<HTMLElement>("#key-error");
    if (error) error.textContent = t("keyRequired");
    return;
  }
  state.profile.ssh.publicKeys = [key];
  state.profile.ssh.passwordAuthentication = false;
  state.profile.transport.mode = mode;
  state.profile.exposure.mode = mode;
  state.profile.transport.install = mode === "tailnet" && !state.report?.snapshot?.tailscale.installed;
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

async function publicKeyIsValid(key: string): Promise<boolean> {
  if (window.go?.main?.App) {
    try {
      await window.go.main.App.ValidatePublicKey(key);
      return true;
    } catch {
      return false;
    }
  }
  const parts = key.trim().split(/\s+/);
  const algorithm = parts[0] ?? "";
  const payload = parts[1] ?? "";
  if (!payload || !/^(ssh-(ed25519|rsa)|ecdsa-sha2-nistp(256|384|521)|sk-ssh-ed25519@openssh\.com|sk-ecdsa-sha2-nistp256@openssh\.com)$/.test(algorithm)) return false;
  try {
    return atob(payload).length > 16;
  } catch {
    return false;
  }
}

function openInstallDialog(): void {
  document.querySelector("#confirm-actions")!.innerHTML = renderConfirmActions(state, t);
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
  const request: DesktopRequest = {
    stage: "apply",
    profile: state.profile,
    planDigest: state.planReport?.plan?.digest ?? "",
    confirmed: true,
    allowSelfCut: false,
    scheduleRisky: false,
    externalVerify: ""
  };
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
  const deadline = Date.now() + 30 * 60 * 1000;
  while (true) {
    if (Date.now() > deadline) {
      state.installState = "failed";
      state.installError = state.language === "zh-CN" ? "安装状态等待超过 30 分钟。管理员进程可能仍在运行；请先检查任务管理器，再重新打开应用检查。" : "Install status timed out after 30 minutes. The elevated process may still be running; inspect it before reopening the app and checking again.";
      renderPage();
      return;
    }
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
  const request: DesktopRequest = { stage, profile: state.profile, planDigest: "", confirmed: false, allowSelfCut: false, scheduleRisky: false, externalVerify: "" };
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
        state.profile = normalizeProfile(profile);
        state.selectedKey = state.profile.ssh.publicKeys[0] ? { label: t("profileImported"), path: "", publicKey: state.profile.ssh.publicKeys[0], generated: false } : undefined;
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
  state.profile.transport.install = state.profile.transport.mode === "tailnet" && !state.report?.snapshot?.tailscale.installed;
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
          return { currentVersion: APP_VERSION, latestVersion: latest, available: isNewerVersion(latest, APP_VERSION), url: value.html_url, channel: "stable" };
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
      state.profile = normalizeProfile(profile);
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
  void file.text().then(async (text) => {
    if (text.includes("PRIVATE KEY")) {
      showToast(state.language === "zh-CN" ? "拒绝导入私钥。请选择 .pub 公钥文件。" : "Private keys are rejected. Choose a .pub file.");
      return;
    }
    for (const candidate of text.split(/\r?\n/).map((line) => line.trim()).filter(Boolean)) {
      if (await publicKeyIsValid(candidate)) {
        selectKey({ label: file.name, path: file.name, publicKey: candidate, generated: false });
        return;
      }
    }
    showToast(t("keyRequired"));
  });
}

async function copyConnectionCommand(): Promise<void> {
  const code = document.querySelector<HTMLElement>(".copy-box code")?.textContent ?? "";
  await navigator.clipboard.writeText(code);
  showToast(t("copied"));
}

function goHome(): void {
  if (state.installState === "waiting-for-permission" || state.installState === "running") {
    showToast(state.language === "zh-CN" ? "安装仍在进行，请等待完成后再离开。" : "Installation is still running. Wait for it to finish before leaving.");
    return;
  }
  state.view = "home";
  state.step = 0;
  state.installState = "idle";
  state.installError = "";
  renderPage();
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
  if (/checksum|sha256/i.test(raw)) return state.language === "zh-CN" ? "下载文件校验失败，没有执行安装。请重试或改用已验证的离线包。" : "Download verification failed. Nothing was installed; retry or use a verified offline bundle.";
  if (/network|timeout|resolve|dns/i.test(raw)) return state.language === "zh-CN" ? "网络暂时不可用。可检查代理、改用官方镜像或离线包后重试。" : "Network unavailable. Check proxy settings, use an explicit trusted mirror, or retry with an offline bundle.";
  return raw || t("errorGeneric");
}

void initialise();
