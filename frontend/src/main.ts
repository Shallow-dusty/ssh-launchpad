import "./styles.css";
import { detectLanguage, translate, type Language, type MessageKey } from "./i18n";
import {
  animateFromCurrent, announce, checked, delay, downloadText, escapeAttribute,
  setText, valueOf
} from "./browser-utils";
import { launchIcon, themeIcon } from "./icons";
import { mockActions, mockPublicKey, mockRun } from "./mock-backend";
import { isNewerVersion, profileToYAML, redactReport } from "./model-utils";
import {
  checkIssues, confirmAckKey, renderAdvanced, renderConfirmActions, renderHome,
  renderWizard, simpleEvent, type InstallState, type WizardMode
} from "./views";
import type { DesktopRequest, ElevatedJob, PersonalCard, Profile, PublicKeyInfo, Report, Stage } from "./types";
import { APP_VERSION } from "./version";

const defaultProfile: Profile = {
  schemaVersion: 1,
  name: "recommended",
  target: { platform: "auto" },
  ssh: { enabled: true, port: 22, publicKeys: [], passwordAuthentication: false },
  transport: { mode: "tailnet", install: false, authKey: "" },
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
  backend: boolean;
  detectedKeys: PublicKeyInfo[];
  selectedKey?: PublicKeyInfo;
  progress: Array<{ kind: string; message: string; actionId?: string }>;
  installState: InstallState;
  installError: string;
  checkError: string;
  verifyError: string;
  activeJob?: ElevatedJob;
  toast: string;
  showNetwork: boolean;
  showKey: boolean;
  keyAttempted: boolean;
} = {
  language: detectLanguage(),
  view: "home",
  mode: "setup",
  step: 0,
  profile: structuredClone(defaultProfile),
  personalCard: {
    displayName: "",
    controllerName: "",
    note: ""
  },
  busy: false,
  backend: Boolean(window.go?.main?.App),
  detectedKeys: [],
  progress: [],
  planError: "",
  installState: "idle",
  installError: "",
  checkError: "",
  verifyError: "",
  toast: "",
  showNetwork: false,
  showKey: false,
  keyAttempted: false
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

// Theme: stored choice wins; otherwise follow the operating system.
function initialTheme(): string {
  const saved = localStorage.getItem("ssh-launchpad-theme");
  if (saved === "dark" || saved === "light") return saved;
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

async function initialise(): Promise<void> {
  document.documentElement.lang = state.language;
  document.documentElement.dataset.theme = initialTheme();
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
  // Engine events are announcement-only; rendering is driven by the job poll
  // (real backend) or by the mock itself, so there is a single render driver
  // per path and no double DOM rebuilds.
  window.runtime?.EventsOn("launchpad:event", (event) => {
    announce(simpleEvent(state.language, event));
  });
  window.runtime?.EventsOn("launchpad:second-instance", () => {
    showToast(t("secondInstance"));
  });
  buildShell();
  renderPage();
}

function buildShell(): void {
  document.querySelector<HTMLDivElement>("#app")!.innerHTML = `
    <div class="app-shell">
      <header class="app-header">
        <button class="brand-button" id="brand-home" aria-label="${escapeAttribute(t("backHome"))}">
          <span class="brand-mark" aria-hidden="true">${launchIcon()}</span>
          <span><strong>${t("appName")}</strong><small>${t("appTagline")}</small></span>
        </button>
        <div class="header-actions">
          ${state.backend ? "" : `<span class="preview-pill">${t("previewMode")}</span>`}
          <div class="lang-switch" role="group" aria-label="${t("languageLabel")}">
            <button id="lang-zh" class="lang-option ${state.language === "zh-CN" ? "active" : ""}" aria-pressed="${state.language === "zh-CN"}">中文</button><button id="lang-en" class="lang-option ${state.language === "en" ? "active" : ""}" aria-pressed="${state.language === "en"}">EN</button>
          </div>
          <button class="icon-button" id="theme-toggle" aria-label="${t("theme")}">${themeIcon()}</button>
        </div>
      </header>
      <main id="workspace" class="workspace" tabindex="-1">
        <div id="announcer" class="sr-only" aria-live="polite"></div>
        <section id="view" class="view"></section>
      </main>
      <div id="toast" class="toast ${state.toast ? "show" : ""}" role="status">${escapeHtmlText(state.toast)}</div>
    </div>
    <dialog id="install-dialog" aria-labelledby="install-dialog-title">
      <form method="dialog" class="dialog-card">
        <h2 id="install-dialog-title">${t("confirmTitle")}</h2>
        <p class="muted">${t("confirmBody")}</p>
        <div id="confirm-actions" class="confirm-list"></div>
        <label class="check-row"><input id="confirm-ack" type="checkbox" /><span id="confirm-ack-label"></span></label>
        <div class="dialog-actions">
          <button value="cancel" class="button secondary">${t("confirmStay")}</button>
          <button id="confirm-install" value="default" class="button primary" disabled>${t("confirmGo")}</button>
        </div>
      </form>
    </dialog>
    <dialog id="confirm-dialog" aria-labelledby="confirm-dialog-title">
      <form method="dialog" class="dialog-card">
        <h2 id="confirm-dialog-title"></h2>
        <p class="muted" id="confirm-dialog-body"></p>
        <div class="dialog-actions">
          <button value="cancel" class="button secondary">${t("confirmStay")}</button>
          <button value="ok" class="button primary">${t("confirmGo")}</button>
        </div>
      </form>
    </dialog>
    <input id="profile-file" class="sr-only" type="file" accept=".yaml,.yml,.json" />
    <input id="card-file" class="sr-only" type="file" accept=".sshlaunchpad-card,.json" />
    <input id="key-file" class="sr-only" type="file" accept=".pub,.txt" />
  `;
  document.querySelector<HTMLElement>(".skip-link")!.textContent = t("skipToContent");
  bindGlobalEvents();
}

function escapeHtmlText(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function bindGlobalEvents(): void {
  document.querySelector("#brand-home")?.addEventListener("click", goHome);
  const switchLanguage = (language: Language) => {
    if (language === state.language) return;
    state.language = language;
    localStorage.setItem("ssh-launchpad-language", language);
    document.documentElement.lang = language;
    state.profile = normalizeProfile(state.profile);
    buildShell();
    renderPage();
    announce(language === "zh-CN" ? "已切换为中文" : "Switched to English");
  };
  document.querySelector("#lang-zh")?.addEventListener("click", () => switchLanguage("zh-CN"));
  document.querySelector("#lang-en")?.addEventListener("click", () => switchLanguage("en"));
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
  document.querySelector<HTMLInputElement>("#card-file")?.addEventListener("change", importPersonalCardFromBrowser);
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
  document.querySelector("#hero-start")?.addEventListener("click", () => startWizard("setup"));
  document.querySelector("#repair-link")?.addEventListener("click", () => startWizard("repair"));
  document.querySelector("#card-import-link")?.addEventListener("click", () => void importPersonalCard());
  document.querySelector("#advanced-link")?.addEventListener("click", () => { state.view = "advanced"; renderPage(); });
  document.querySelector("#wizard-back")?.addEventListener("click", goHome);
  document.querySelector("#advanced-back")?.addEventListener("click", goHome);
  document.querySelector("#run-check")?.addEventListener("click", () => void runCheck());
  document.querySelector("#check-continue")?.addEventListener("click", () => void onCheckContinue());
  document.querySelector("#plan-back")?.addEventListener("click", () => { state.step = 0; state.installState = "idle"; renderPage(); });
  document.querySelector("#plan-retry")?.addEventListener("click", () => void runPlanStage());
  document.querySelector("#change-network")?.addEventListener("click", () => {
    state.showNetwork = !state.showNetwork;
    renderPage();
  });
  document.querySelector("#change-key")?.addEventListener("click", () => {
    state.showKey = !state.showKey;
    renderPage();
  });
  document.querySelectorAll<HTMLInputElement>('input[name="network-mode"]').forEach((input) => input.addEventListener("change", () => {
    setNetworkMode(input.value === "lan" ? "lan" : "tailnet");
  }));
  document.querySelectorAll<HTMLInputElement>('input[name="controller-key"]').forEach((input) => input.addEventListener("change", () => {
    state.selectedKey = state.detectedKeys[Number(input.value)];
    if (state.selectedKey) state.profile.ssh.publicKeys = [state.selectedKey.publicKey];
    state.keyAttempted = false;
    void runPlanStage();
  }));
  bindPublicKeyInput();
  document.querySelector("#import-key")?.addEventListener("click", () => void importPublicKey());
  document.querySelector("#generate-key")?.addEventListener("click", () => void generatePublicKey());
  document.querySelector("#export-pairing")?.addEventListener("click", () => void exportPairing());
  document.querySelector("#open-install")?.addEventListener("click", () => {
    // No key yet: open the picker panel instead of failing after the fact.
    if (!state.selectedKey && !state.profile.ssh.publicKeys[0]) {
      state.showKey = true;
      renderPage();
      document.querySelector<HTMLTextAreaElement>("#public-key")?.focus();
      return;
    }
    openInstallDialog();
  });
  document.querySelector("#test-now")?.addEventListener("click", () => void runVerify());
  document.querySelector("#verify-again")?.addEventListener("click", () => void runVerify());
  document.querySelector("#copy-command")?.addEventListener("click", copyConnectionCommand);
  document.querySelector("#finish")?.addEventListener("click", goHome);
  document.querySelector("#import-profile")?.addEventListener("click", () => void importProfile());
  document.querySelector("#export-profile")?.addEventListener("click", () => void exportProfile());
  bindAdvancedAutoApply();
  document.querySelector("#import-personal-card-advanced")?.addEventListener("click", () => void importPersonalCard());
  document.querySelector("#export-personal-card")?.addEventListener("click", () => void exportPersonalCard());
  document.querySelector("#advanced-check")?.addEventListener("click", () => void runAdvancedStage("check"));
  document.querySelector("#advanced-plan")?.addEventListener("click", () => void runAdvancedStage("plan"));
  document.querySelector("#export-report-advanced")?.addEventListener("click", () => void exportReport());
  document.querySelector("#check-update")?.addEventListener("click", () => void checkForUpdate());
  document.querySelector("#rollback-last")?.addEventListener("click", () => void rollbackLast());
}

// Pasted keys validate and rebuild the plan as the user settles (debounced),
// instead of waiting for a blur — the page never sits stale after a paste.
let keyInputTimer: ReturnType<typeof setTimeout> | undefined;
function bindPublicKeyInput(): void {
  const textarea = document.querySelector<HTMLTextAreaElement>("#public-key");
  if (!textarea) return;
  textarea.addEventListener("input", () => {
    const value = textarea.value.trim();
    clearTimeout(keyInputTimer);
    if (!value) {
      state.selectedKey = undefined;
      state.profile.ssh.publicKeys = [];
      state.keyAttempted = false;
      void runPlanStage();
      return;
    }
    keyInputTimer = setTimeout(() => {
      void (async () => {
        if (await publicKeyIsValid(value)) {
          state.selectedKey = { label: t("pasteKey"), path: "", publicKey: value, generated: false };
          state.profile.ssh.publicKeys = [value];
          state.keyAttempted = false;
          void runPlanStage();
        } else {
          state.keyAttempted = true;
          state.selectedKey = undefined;
          state.profile.ssh.publicKeys = [];
          renderPage();
          document.querySelector<HTMLTextAreaElement>("#public-key")?.focus();
        }
      })();
    }, 450);
  });
}

function startWizard(mode: WizardMode): void {
  clearTimeout(keyInputTimer);
  keyInputTimer = undefined;
  state.view = "wizard";
  state.mode = mode;
  state.step = 0;
  state.report = undefined;
  state.planReport = undefined;
  state.planError = "";
  state.verifyReport = undefined;
  state.progress = [];
  state.installState = "idle";
  state.installError = "";
  state.checkError = "";
  state.verifyError = "";
  state.showNetwork = false;
  state.showKey = false;
  state.keyAttempted = false;
  renderPage();
  void runCheck();
}

async function onCheckContinue(): Promise<void> {
  // A machine with nothing to fix (or a healthy repair diagnosis) skips the
  // prepare step and goes straight to verification — matching the CTA label.
  if (state.report?.snapshot && checkIssues(state.report.snapshot, state.profile).length === 0) {
    await runVerify();
    return;
  }
  await enterPlanStep();
}

async function enterPlanStep(): Promise<void> {
  state.step = 1;
  state.installState = "idle";
  state.installError = "";
  const firstDetected = state.detectedKeys[0];
  if (state.mode === "setup" && !state.selectedKey && !state.profile.ssh.publicKeys[0] && firstDetected) {
    state.selectedKey = firstDetected;
    state.profile.ssh.publicKeys = [firstDetected.publicKey];
  }
  // The key picker opens on its own only when a key is genuinely missing.
  state.showKey = state.mode === "setup" && !state.selectedKey && !state.profile.ssh.publicKeys[0];
  state.showNetwork = false;
  await runPlanStage();
}

async function runPlanStage(): Promise<void> {
  if (state.busy) return;
  state.busy = true;
  state.planError = "";
  renderPage();
  try {
    state.planReport = await runStage("plan");
  } catch (error) {
    state.planReport = undefined;
    state.planError = friendlyError(error);
  } finally {
    state.busy = false;
    renderPage();
  }
}

function setNetworkMode(mode: "tailnet" | "lan"): void {
  state.profile.transport.mode = mode;
  state.profile.exposure.mode = mode;
  state.profile.transport.install = mode === "tailnet" && !state.report?.snapshot?.tailscale.installed;
  if (mode === "lan") state.profile.transport.authKey = "";
  void runPlanStage();
}

async function runCheck(): Promise<void> {
  if (state.busy) return;
  state.busy = true;
  state.checkError = "";
  renderPage();
  try {
    state.report = await runStage("check");
    // Match the CLI wizard's guided default: a fresh recommended setup may
    // install the optional transport during the reviewed Apply. Imported
    // profiles and setup cards keep their explicit install choice.
    const guidedDefault = state.profile.name === "recommended" || state.profile.name === "default";
    if (state.mode === "setup" && guidedDefault && state.profile.transport.mode === "tailnet"
      && !state.report.snapshot?.tailscale.installed) {
      state.profile.transport.install = true;
    }
  } catch (error) {
    state.checkError = friendlyError(error);
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
  setText("#confirm-ack-label", t(confirmAckKey(state)));
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
  const plan = state.planReport?.plan;
  const request: DesktopRequest = {
    stage: "apply",
    profile: state.profile,
    planDigest: plan?.digest ?? "",
    confirmed: true,
    allowSelfCut: false,
    scheduleRisky: false,
    externalVerify: "",
    planNoChanges: plan?.noChanges ?? false,
    planNeedsElevation: (plan?.actions ?? []).some((action) => action.mutating && action.requiresElevation)
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
  let renderedFingerprint = "";
  while (true) {
    if (Date.now() > deadline) {
      state.installState = "failed";
      state.installError = t("installTimeout");
      renderPage();
      return;
    }
    const job = await window.go!.main!.App!.ElevatedApplyStatus(id);
    state.activeJob = job;
    state.progress = job.events ?? [];
    state.installState = job.state === "waiting-for-permission" ? "waiting-for-permission" : job.state === "running" ? "running" : job.state;
    const fingerprint = `${state.installState}|${state.progress.length}`;
    if (fingerprint !== renderedFingerprint) {
      renderedFingerprint = fingerprint;
      renderPage();
    }
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
    state.installError = "";
    renderPage();
    return;
  }
  if (job.state === "failed" || !job.report?.success) {
    state.installState = "failed";
    state.installError = friendlyReportError(job.error || job.report?.error || "", job.report);
    renderPage();
    return;
  }
  state.installState = "completed";
  state.report = job.report;
  localStorage.setItem("ssh-launchpad-demo-ready", "true");
  void runVerify();
}

async function runVerify(): Promise<void> {
  if (state.busy) return;
  state.step = 2;
  state.busy = true;
  state.verifyError = "";
  renderPage();
  try {
    // A failed verification must surface as a failure, never backfilled with
    // the stale Apply-time report.
    state.verifyReport = await runStage("verify");
  } catch (error) {
    state.verifyReport = undefined;
    state.verifyError = friendlyError(error);
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
  if (state.busy) return;
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
  state.keyAttempted = false;
  if (!state.detectedKeys.some((existing) => existing.publicKey === key.publicKey)) state.detectedKeys.push(key);
  renderPage();
  if (state.view === "wizard" && state.step === 1) void runPlanStage();
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
  if (window.go?.main?.App) {
    const path = await window.go.main.App.ExportProfile(state.profile);
    if (path) showToast(t("profileExported"));
    return;
  }
  downloadText(`${state.profile.name}.ssh-launchpad.yaml`, profileToYAML(state.profile), "text/yaml;charset=utf-8");
  showToast(t("profileExported"));
}

async function importPersonalCard(): Promise<void> {
  try {
    if (window.go?.main?.App) {
      const card = await window.go.main.App.ImportPersonalCard();
      if (card.schemaVersion) await applyPersonalCard(card);
    } else {
      document.querySelector<HTMLInputElement>("#card-file")!.click();
    }
  } catch (error) {
    showToast(friendlyError(error));
  }
}

async function exportPersonalCard(): Promise<void> {
  const card = buildPersonalCard();
  const error = await validatePersonalCardClient(card);
  if (error) {
    showToast(error);
    return;
  }
  if (window.go?.main?.App) {
    const path = await window.go.main.App.ExportPersonalCard(card);
    if (path) showToast(t("cardExported"));
    return;
  }
  downloadText(`${safeCardFilename(card.displayName)}.sshlaunchpad-card`, `${JSON.stringify(card, null, 2)}\n`, "application/json;charset=utf-8");
  showToast(t("cardExported"));
}

function buildPersonalCard(): PersonalCard {
  return {
    schemaVersion: 1,
    kind: "ssh-launchpad-personal-card",
    displayName: state.personalCard.displayName.trim(),
    controllerName: state.personalCard.controllerName.trim() || undefined,
    note: state.personalCard.note.trim() || undefined,
    ssh: {
      port: state.profile.ssh.port,
      publicKeys: [...state.profile.ssh.publicKeys]
    },
    tailscale: {
      mode: state.profile.transport.mode === "lan" ? "lan" : "tailnet",
      install: state.profile.transport.mode === "tailnet" && state.profile.transport.install,
      authKey: state.profile.transport.authKey?.trim() || undefined
    }
  };
}

async function applyPersonalCard(card: PersonalCard): Promise<void> {
  const error = await validatePersonalCardClient(card);
  if (error) throw new Error(error);
  state.personalCard = {
    displayName: card.displayName.trim(),
    controllerName: card.controllerName?.trim() ?? "",
    note: card.note?.trim() ?? ""
  };
  state.profile = normalizeProfile({
    ...structuredClone(defaultProfile),
    name: card.displayName.trim(),
    ssh: {
      ...structuredClone(defaultProfile.ssh),
      port: card.ssh.port,
      publicKeys: [...card.ssh.publicKeys]
    },
    transport: {
      mode: card.tailscale.mode,
      install: card.tailscale.mode === "tailnet" && card.tailscale.install,
      authKey: card.tailscale.mode === "tailnet" ? card.tailscale.authKey?.trim() ?? "" : ""
    },
    exposure: {
      ...structuredClone(defaultProfile.exposure),
      mode: card.tailscale.mode
    },
    labels: {
      experience: "guided",
      cardDisplayName: card.displayName.trim(),
      ...(card.controllerName?.trim() ? { cardControllerName: card.controllerName.trim() } : {}),
      ...(card.note?.trim() ? { cardNote: card.note.trim() } : {})
    }
  });
  const firstKey = card.ssh.publicKeys[0]!;
  state.selectedKey = { label: card.controllerName?.trim() || t("cardTitle"), path: "", publicKey: firstKey, generated: false };
  for (const publicKey of card.ssh.publicKeys) {
    if (!state.detectedKeys.some((existing) => existing.publicKey === publicKey)) {
      state.detectedKeys.push({ label: card.controllerName?.trim() || t("cardTitle"), path: "", publicKey, generated: false });
    }
  }
  showToast(t("cardImported"));
  startWizard("setup");
}

async function validatePersonalCardClient(card: PersonalCard): Promise<string> {
  if (card.schemaVersion !== 1 || card.kind !== "ssh-launchpad-personal-card") return t("cardInvalid");
  if (!card.displayName?.trim() || card.displayName.trim().length > 128) return t("cardNameRequired");
  if (card.controllerName && card.controllerName.length > 128) return t("cardInvalid");
  if (card.note && card.note.length > 1024) return t("cardInvalid");
  if (!Number.isInteger(card.ssh?.port) || card.ssh.port < 1 || card.ssh.port > 65535) return t("cardInvalid");
  if (!Array.isArray(card.ssh?.publicKeys) || card.ssh.publicKeys.length === 0 || card.ssh.publicKeys.length > 128) return t("keyMissingTitle");
  for (const key of card.ssh.publicKeys) {
    if (key.includes("PRIVATE KEY") || !(await publicKeyIsValid(key))) return t("cardInvalid");
  }
  if (!["tailnet", "lan"].includes(card.tailscale?.mode)) return t("cardInvalid");
  if (card.tailscale.mode === "lan" && card.tailscale.authKey) return t("cardInvalid");
  if (card.tailscale.authKey && (card.tailscale.authKey.length > 4096 || !card.tailscale.authKey.startsWith("tskey-auth-") || /[\r\n\0]/.test(card.tailscale.authKey))) return t("cardInvalid");
  return "";
}

function safeCardFilename(value: string): string {
  return value.trim().replace(/[<>:"/\\|?*\x00-\x1f]/g, "-").replace(/[. ]+$/g, "") || "ssh-launchpad-setup";
}

// Advanced settings apply live: every field writes straight into state on
// input, so there is no save button and no hidden save side effect.
function bindAdvancedAutoApply(): void {
  if (!document.querySelector("#target-platform")) return;
  const applied = () => setText("#advanced-status", t("applied"));
  const onInput = (id: string, apply: (value: string) => void) => {
    document.querySelector(`#${id}`)?.addEventListener("input", (event) => {
      apply((event.currentTarget as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement).value);
      applied();
    });
  };
  onInput("card-display-name", (value) => { state.personalCard.displayName = value; });
  onInput("card-controller-name", (value) => { state.personalCard.controllerName = value; });
  onInput("card-note", (value) => { state.personalCard.note = value; });
  onInput("card-tailscale-auth-key", () => { syncTransport(); });
  onInput("target-platform", (value) => { state.profile.target.platform = value; });
  onInput("ssh-port", (value) => {
    const port = Number(value);
    if (Number.isInteger(port) && port >= 1 && port <= 65535) state.profile.ssh.port = port;
  });
  onInput("transport-mode", () => { syncTransport(); });
  onInput("exposure-mode", (value) => { state.profile.exposure.mode = value; });
  onInput("download-strategy", (value) => { state.profile.download.strategy = value; });
  onInput("advanced-keys", (value) => {
    state.profile.ssh.publicKeys = value.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  });
  document.querySelector("#prevent-self-cut")?.addEventListener("change", () => {
    state.profile.safety.preventSelfCut = checked("prevent-self-cut");
    applied();
  });
  document.querySelector("#auto-rollback")?.addEventListener("change", () => {
    state.profile.safety.autoRollback = checked("auto-rollback");
    applied();
  });
}

function syncTransport(): void {
  state.profile.transport.mode = valueOf("transport-mode");
  state.profile.transport.install = state.profile.transport.mode === "tailnet" && !state.report?.snapshot?.tailscale.installed;
  state.profile.transport.authKey = state.profile.transport.mode === "tailnet" ? valueOf("card-tailscale-auth-key").trim() : "";
}

async function exportReport(): Promise<void> {
  if (!state.report) {
    showToast(t("reportMissing"));
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
      showToast(t("updateAvailable", { version: info.latestVersion }));
      window.open(info.url, "_blank", "noopener,noreferrer");
    } else {
      showToast(t("updateLatest"));
    }
  } catch (error) {
    showToast(friendlyError(error));
  }
}

async function rollbackLast(): Promise<void> {
  if (!state.report?.journalPath || !window.go?.main?.App) return;
  if (!(await confirmDialog(t("rollbackLast"), t("rollbackConfirmBody")))) return;
  try {
    const report = await window.go.main.App.Rollback(state.report.journalPath);
    state.report = report;
    showToast(report.success ? t("verdictReady") : t("errorGeneric"));
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
      showToast(t("browserJsonOnly"));
    }
  });
}

function importPersonalCardFromBrowser(event: Event): void {
  const input = event.currentTarget as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  void file.text().then(async (text) => {
    try {
      if (text.includes("PRIVATE KEY")) {
        throw new Error(t("keyRejectedPrivate"));
      }
      const card = JSON.parse(text) as PersonalCard;
      await applyPersonalCard(card);
    } catch (error) {
      const message = friendlyError(error);
      showToast(message === t("errorGeneric") ? t("cardInvalid") : message);
    } finally {
      input.value = "";
    }
  });
}

function importKeyFromBrowser(event: Event): void {
  const file = (event.currentTarget as HTMLInputElement).files?.[0];
  if (!file) return;
  void file.text().then(async (text) => {
    if (text.includes("PRIVATE KEY")) {
      showToast(t("keyRejectedPrivate"));
      return;
    }
    for (const candidate of text.split(/\r?\n/).map((line) => line.trim()).filter(Boolean)) {
      if (await publicKeyIsValid(candidate)) {
        selectKey({ label: file.name, path: file.name, publicKey: candidate, generated: false });
        return;
      }
    }
    state.keyAttempted = true;
    renderPage();
  });
}

async function copyConnectionCommand(): Promise<void> {
  const code = document.querySelector<HTMLElement>(".command-box code")?.textContent ?? "";
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(code);
    } else {
      const fallback = document.createElement("textarea");
      fallback.value = code;
      fallback.setAttribute("readonly", "true");
      fallback.style.position = "fixed";
      fallback.style.opacity = "0";
      document.body.appendChild(fallback);
      fallback.select();
      if (!document.execCommand("copy")) throw new Error("clipboard unavailable");
      fallback.remove();
    }
    showToast(t("copied"));
  } catch {
    showToast(t("errorGeneric"));
  }
}

function goHome(): void {
  if (state.installState === "waiting-for-permission" || state.installState === "running") {
    showToast(t("installingBody"));
    return;
  }
  clearTimeout(keyInputTimer);
  keyInputTimer = undefined;
  state.view = "home";
  state.step = 0;
  state.installState = "idle";
  state.installError = "";
  state.planError = "";
  renderPage();
}

async function mockElevatedApply(request: DesktopRequest): Promise<ElevatedJob> {
  await delay(250);
  const mode = new URLSearchParams(location.search).get("mock");
  const attempt = Number(sessionStorage.getItem("ssh-launchpad-mock-attempt") ?? "0") + 1;
  sessionStorage.setItem("ssh-launchpad-mock-attempt", String(attempt));
  if (mode === "uac-cancel" && attempt === 1) return { id: "mock", state: "cancelled", error: t("installCancelled") };
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

function confirmDialog(title: string, body: string): Promise<boolean> {
  return new Promise((resolve) => {
    const dialog = document.querySelector<HTMLDialogElement>("#confirm-dialog")!;
    setText("#confirm-dialog-title", title);
    setText("#confirm-dialog-body", body);
    const onClose = () => {
      dialog.removeEventListener("close", onClose);
      resolve(dialog.returnValue === "ok");
    };
    dialog.addEventListener("close", onClose);
    dialog.showModal();
  });
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

// Stage reports carry a stable exit code, so known failure classes map to
// guidance instead of fragile error-text matching.
function friendlyReportError(raw: string, report?: Report): string {
  switch (report?.exitCode) {
    case 8:
      return t("errorDownload");
    case 6:
      return t("errorSelfCut");
    case 5:
      return t("errorPlanChanged");
    default:
      return raw || t("errorGeneric");
  }
}

function friendlyError(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error ?? "");
  if (/checksum|sha256/i.test(raw)) return t("errorDownload");
  if (/network|timeout|resolve|dns/i.test(raw)) return t("errorNetwork");
  return raw || t("errorGeneric");
}

void initialise();
