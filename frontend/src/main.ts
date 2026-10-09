import "./styles.css";
import { translate, type Language, type MessageKey } from "./i18n";
import { animateFromCurrent, announce, escapeAttribute } from "./browser-utils";
import { launchIcon, themeIcon } from "./icons";
import { createBackend } from "./backend";
import { createState } from "./state";
import { normalizeProfile } from "./profile";
import { createFeedback } from "./feedback";
import { createWizard } from "./wizard";
import { createTransfers } from "./transfers";
import { createAdvanced } from "./advanced";
import { renderAdvanced, renderHome, renderWizard, simpleEvent } from "./views";

const backend = createBackend();
const state = createState(backend.kind === "desktop");
const t = (key: MessageKey, values: Record<string, string | number> = {}) => translate(state.language, key, values);
const feedback = createFeedback(state, t);
const context = { state, backend, t, renderPage, feedback };
const wizard = createWizard(context);
const transfers = createTransfers(context, wizard);
const advanced = createAdvanced(context, wizard.runStage);
const { showToast, friendlyError, copyConnectionCommand } = feedback;
const {
  bindPublicKeyInput, startWizard, onCheckContinue, runPlanStage, setNetworkMode,
  runCheck, openInstallDialog, beginSafeInstall, runVerify, rollbackLast, goHome
} = wizard;
const {
  importPublicKey, generatePublicKey, exportPairing, importProfile, exportProfile,
  importPersonalCard, exportPersonalCard, importProfileFromBrowser,
  importPersonalCardFromBrowser, importKeyFromBrowser
} = transfers;
const { runAdvancedStage, bindAdvancedAutoApply, exportReport, checkForUpdate } = advanced;

// Theme: stored choice wins; otherwise follow the operating system.
function initialTheme(): string {
  const saved = localStorage.getItem("ssh-launchpad-theme");
  if (saved === "dark" || saved === "light") return saved;
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

async function initialise(): Promise<void> {
  document.documentElement.lang = state.language;
  document.documentElement.dataset.theme = initialTheme();
  try {
    state.profile = await backend.defaultProfile();
    if (backend.kind === "desktop") {
      state.profile.name = "recommended";
      state.profile.labels = { ...state.profile.labels, experience: "guided" };
    }
    state.detectedKeys = await backend.discoverPublicKeys() ?? [];
  } catch (error) {
    state.toast = friendlyError(error);
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

let renderedLocation = "";
function renderPage(): void {
  const location = `${state.view}:${state.step}`;
  const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement.id : "";
  const view = document.querySelector<HTMLElement>("#view")!;
  animateFromCurrent(view);
  if (state.view === "home") view.innerHTML = renderHome(state, t);
  if (state.view === "wizard") view.innerHTML = renderWizard(state, t);
  if (state.view === "advanced") view.innerHTML = renderAdvanced(state, t);
  bindPageEvents();
  const heading = view.querySelector<HTMLElement>("h1");
  heading?.setAttribute("tabindex", "-1");
  heading?.setAttribute("id", "view-heading");
  if (location !== renderedLocation) {
    if (renderedLocation) heading?.focus({ preventScroll: true });
    renderedLocation = location;
  } else if (previousFocus) {
    document.getElementById(previousFocus)?.focus({ preventScroll: true });
  }
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
  document.querySelector("#copy-command")?.addEventListener("click", () => void copyConnectionCommand());
  document.querySelector("#copy-handoff")?.addEventListener("click", () => void copyConnectionCommand(true));
  document.querySelector("#review-remaining")?.addEventListener("click", () => { state.step = 1; state.installState = "idle"; void runPlanStage(); });
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

void initialise();
