import "./styles.css";
import type { DesktopRequest, PlanAction, Profile, Report, Snapshot, Stage } from "./types";

const defaultProfile: Profile = {
  schemaVersion: 1,
  name: "default",
  target: { platform: "auto" },
  ssh: { enabled: true, port: 22, publicKeys: [], passwordAuthentication: false },
  transport: { mode: "tailnet", install: false },
  exposure: { mode: "tailnet", customCidrs: [] },
  download: { strategy: "official", mirrorBaseUrl: "", proxyUrl: "", offlineBundle: "", cacheDir: "", retries: 3 },
  safety: { confirmHighRisk: true, preventSelfCut: true, scheduledDelaySeconds: 20, autoRollback: true },
  advanced: { windowsSshService: "sshd", linuxSshService: "auto", macosSshLabel: "com.openssh.sshd", stateDir: "" },
  labels: {}
};

const state: {
  profile: Profile;
  report?: Report;
  busy: boolean;
  activeView: string;
  progress: Array<{ kind: string; message: string; actionId?: string }>;
} = {
  profile: structuredClone(defaultProfile),
  busy: false,
  activeView: "status",
  progress: []
};

document.querySelector<HTMLDivElement>("#app")!.innerHTML = `
  <div class="shell">
    <aside class="sidebar" aria-label="Primary navigation">
      <div class="brand">
        <span class="brand-mark" aria-hidden="true">
          <svg viewBox="0 0 32 32" role="img"><path d="M6 10.5 16 5l10 5.5v11L16 27 6 21.5Z"/><path d="m11 15 3 3-3 3m5 0h5"/></svg>
        </span>
        <span><strong>SSH Launchpad</strong><small>Control access, calmly.</small></span>
      </div>
      <nav>
        ${navButton("status", "Status", statusIcon())}
        ${navButton("plan", "Plan", planIcon())}
        ${navButton("progress", "Progress", progressIcon())}
        ${navButton("verify", "Verify", verifyIcon())}
        ${navButton("recovery", "Recovery", recoveryIcon())}
        ${navButton("advanced", "Advanced", advancedIcon())}
      </nav>
      <div class="sidebar-foot">
        <span class="version">v0.1.0</span>
        <button class="icon-button" id="theme-toggle" aria-label="Toggle color theme">${themeIcon()}</button>
      </div>
    </aside>

    <main id="workspace" class="workspace" tabindex="-1">
      <header class="topbar material">
        <div>
          <p class="eyebrow">CHECK · PLAN · APPLY · VERIFY</p>
          <h1 id="view-title">Host status</h1>
        </div>
        <div class="top-actions">
          <span id="backend-mode" class="backend-pill">Detecting core…</span>
          <button class="button secondary" id="export-report" disabled>Export report</button>
          <button class="button primary" id="run-check">Run check</button>
        </div>
      </header>

      <div id="announcer" class="sr-only" aria-live="polite"></div>
      <section id="view" class="view" aria-labelledby="view-title"></section>
    </main>
  </div>

  <dialog id="apply-dialog" aria-labelledby="apply-title">
    <form method="dialog" class="dialog-card">
      <div class="dialog-icon danger" aria-hidden="true">${warningIcon()}</div>
      <div>
        <p class="eyebrow danger-text">HIGH-IMPACT ACTION</p>
        <h2 id="apply-title">Confirm the exact changes</h2>
        <p id="apply-summary" class="muted"></p>
      </div>
      <div id="apply-actions" class="confirm-list"></div>
      <label class="check-row">
        <input id="apply-ack" type="checkbox" />
        <span>I reviewed the actions, rollback coverage, and control-channel risk.</span>
      </label>
      <label class="check-row">
        <input id="schedule-risky" type="checkbox" />
        <span>Schedule self-cut-sensitive actions after a delay.</span>
      </label>
      <label class="field">
        <span>Independent verification target</span>
        <input id="external-verify" type="text" inputmode="url" autocomplete="off" placeholder="controller-visible.example:22" />
        <small>Required for scheduled self-cut-sensitive work. It must be reachable before scheduling and rechecked from the controller afterward.</small>
      </label>
      <div class="dialog-actions">
        <button value="cancel" class="button secondary">Cancel</button>
        <button id="confirm-apply" value="default" class="button danger-button" disabled>Apply changes</button>
      </div>
    </form>
  </dialog>
`;

const titles: Record<string, string> = {
  status: "Host status",
  plan: "Change plan",
  progress: "Execution progress",
  verify: "Layered verification",
  recovery: "Recovery & rollback",
  advanced: "Advanced configuration"
};

function navButton(view: string, label: string, icon: string): string {
  return `<button class="nav-item ${view === "status" ? "active" : ""}" data-view="${view}" aria-current="${view === "status" ? "page" : "false"}">${icon}<span>${label}</span></button>`;
}

async function initialise(): Promise<void> {
  const backend = window.go?.main?.App;
  if (backend) {
    try {
      state.profile = await backend.DefaultProfile();
      setText("#backend-mode", "Native core");
      document.querySelector("#backend-mode")?.classList.add("online");
    } catch {
      setText("#backend-mode", "Native core unavailable");
    }
  } else {
    setText("#backend-mode", "Interactive prototype");
  }
  window.runtime?.EventsOn("launchpad:event", (event) => {
    state.progress.push(event);
    announce(`${event.kind}: ${event.message}`);
    if (state.activeView === "progress") render();
  });
  bindEvents();
  render();
}

function bindEvents(): void {
  document.querySelectorAll<HTMLButtonElement>(".nav-item").forEach((button) => {
    button.addEventListener("pointerdown", () => button.classList.add("pressed"));
    button.addEventListener("pointerup", () => button.classList.remove("pressed"));
    button.addEventListener("pointercancel", () => button.classList.remove("pressed"));
    button.addEventListener("click", () => {
      state.activeView = button.dataset.view ?? "status";
      document.querySelectorAll(".nav-item").forEach((item) => {
        const active = item === button;
        item.classList.toggle("active", active);
        item.setAttribute("aria-current", active ? "page" : "false");
      });
      setText("#view-title", titles[state.activeView] ?? "SSH Launchpad");
      render(true);
    });
  });
  document.querySelector("#run-check")?.addEventListener("click", () => runStage("check"));
  document.querySelector("#export-report")?.addEventListener("click", exportReport);
  document.querySelector("#theme-toggle")?.addEventListener("click", toggleTheme);
  const ack = document.querySelector<HTMLInputElement>("#apply-ack")!;
  ack.addEventListener("change", () => {
    document.querySelector<HTMLButtonElement>("#confirm-apply")!.disabled = !ack.checked;
  });
  document.querySelector("#confirm-apply")?.addEventListener("click", (event) => {
    event.preventDefault();
    if (!ack.checked) return;
    document.querySelector<HTMLDialogElement>("#apply-dialog")!.close();
    runStage(
      "apply",
      true,
      document.querySelector<HTMLInputElement>("#schedule-risky")!.checked,
      document.querySelector<HTMLInputElement>("#external-verify")!.value.trim()
    );
  });
}

function render(animate = false): void {
  const view = document.querySelector<HTMLElement>("#view")!;
  if (animate) animateFromCurrent(view);
  switch (state.activeView) {
    case "plan":
      view.innerHTML = renderPlan();
      bindPlanEvents();
      break;
    case "progress":
      view.innerHTML = renderProgress();
      break;
    case "verify":
      view.innerHTML = renderVerify();
      document.querySelector("#run-verify")?.addEventListener("click", () => runStage("verify"));
      break;
    case "recovery":
      view.innerHTML = renderRecovery();
      bindRecoveryEvents();
      break;
    case "advanced":
      view.innerHTML = renderAdvanced();
      bindAdvancedEvents();
      break;
    default:
      view.innerHTML = renderStatus();
      document.querySelector("#status-plan")?.addEventListener("click", () => runStage("plan"));
  }
}

function renderStatus(): string {
  const snapshot = state.report?.snapshot;
  return `
    <div class="hero-grid">
      <article class="hero-card material">
        <div class="hero-copy">
          <p class="eyebrow">CURRENT HOST</p>
          <h2>${escapeHtml(snapshot?.hostname ?? "Ready to inspect")}</h2>
          <p>${snapshot ? `${escapeHtml(snapshot.platform)} · ${escapeHtml(snapshot.arch)} · ${snapshot.isAdministrator ? "elevated" : "standard token"}` : "Run a read-only Check to detect the host, platform, services, network, and exposure."}</p>
        </div>
        <div class="host-orbit" aria-hidden="true"><span></span><i></i><b></b></div>
      </article>
      <article class="score-card ${snapshot ? healthClass(snapshot) : ""}">
        <p class="eyebrow">READINESS</p>
        <strong>${snapshot ? healthScore(snapshot) : "—"}</strong>
        <span>${snapshot ? healthLabel(snapshot) : "Not checked"}</span>
      </article>
    </div>

    <div class="section-heading"><div><p class="eyebrow">LAYERED STATE</p><h2>Do not blend these signals</h2></div><button id="status-plan" class="button secondary">Build plan</button></div>
    <div class="status-grid">
      ${statusCard("Platform", snapshot ? `${snapshot.platform} / ${snapshot.arch}` : "Unknown", Boolean(snapshot), "Host and WSL are always separate targets.", platformIcon())}
      ${statusCard("Secure transport", snapshot?.tailscale.online ? `Online · ${snapshot.tailscale.ip ?? "Tailnet"}` : snapshot?.tailscale.installed ? "Installed · offline" : "Not detected", Boolean(snapshot?.tailscale.online), "Optional transport; SSH remains independently testable.", transportIcon())}
      ${statusCard("SSH service", snapshot?.sshService.running ? `${snapshot.sshService.name ?? "sshd"} · running` : "Not running", Boolean(snapshot?.sshService.running), `Listener: ${snapshot?.sshPort ?? "—"}`, serviceIcon())}
      ${statusCard("Firewall", snapshot?.firewall.provider ? `${snapshot.firewall.provider}` : "No provider detected", Boolean(snapshot?.firewall.ports?.length), snapshot?.firewall.ports?.length ? `Ports: ${snapshot.firewall.ports.join(", ")}` : "Port and scope are verified together.", firewallIcon())}
      ${statusCard("Network sources", snapshot?.network.githubDns && snapshot?.network.tailscaleDns ? "DNS paths resolve" : "One or more paths unavailable", Boolean(snapshot?.network.githubDns && snapshot?.network.tailscaleDns), snapshot?.network.proxySet ? "Explicit proxy detected" : "No process proxy detected", networkIcon())}
      ${statusCard("Configuration", snapshot?.sshConfigValid ? "Syntax valid" : "Needs verification", Boolean(snapshot?.sshConfigValid), "Verify stays read-only and does not elevate.", configIcon())}
    </div>
    ${renderWarnings(snapshot?.warnings)}
  `;
}

function renderPlan(): string {
  const plan = state.report?.plan;
  const actions = plan?.actions ?? [];
  return `
    <div class="summary-strip">
      ${metric("Actions", String(actions.length))}
      ${metric("Highest risk", plan?.highestRisk ?? "—")}
      ${metric("Self-cut", plan?.selfCutDetected ? "Detected" : "Clear")}
      ${metric("Elevation", actions.some((action) => action.requiresElevation) ? "Required" : "No")}
    </div>
    <article class="panel material">
      <div class="section-heading">
        <div><p class="eyebrow">READ-ONLY DIFF</p><h2>${plan?.noChanges ? "Desired state already matches" : "What Apply would change"}</h2></div>
        <button id="refresh-plan" class="button secondary">Refresh plan</button>
      </div>
      ${actions.length ? `<div class="action-list">${actions.map(actionRow).join("")}</div>` : emptyState("No plan yet", "Run Plan to compare the current host with your profile.")}
      <div class="panel-footer">
        <span>${plan?.selfCutDetected ? "A control-channel risk must be scheduled or externally verified." : "Apply remains locked until this exact diff is confirmed."}</span>
        <button id="open-apply" class="button ${plan?.selfCutDetected ? "danger-button" : "primary"}" ${actions.length === 0 || state.busy ? "disabled" : ""}>Review & apply</button>
      </div>
    </article>
    ${renderWarnings(plan?.warnings)}
  `;
}

function renderProgress(): string {
  const events = state.progress;
  const results = state.report?.results ?? [];
  return `
    <article class="panel material">
      <div class="section-heading"><div><p class="eyebrow">LIVE EXECUTION</p><h2>${state.busy ? "Working through the plan" : "Execution timeline"}</h2></div><span class="activity-dot ${state.busy ? "active" : ""}" aria-label="${state.busy ? "running" : "idle"}"></span></div>
      <ol class="timeline">
        ${events.length ? events.map((event) => `<li class="${escapeHtml(event.kind)}"><span></span><div><strong>${escapeHtml(event.actionId ?? event.kind)}</strong><p>${escapeHtml(event.message)}</p></div></li>`).join("") : `<li class="idle"><span></span><div><strong>No actions running</strong><p>Apply progress and servicing output will appear here.</p></div></li>`}
      </ol>
      ${results.length ? `<details class="log-details"><summary>Structured action results</summary><pre>${escapeHtml(JSON.stringify(results, null, 2))}</pre></details>` : ""}
    </article>
  `;
}

function renderVerify(): string {
  const snapshot = state.report?.stage === "verify" ? state.report.snapshot : undefined;
  const checks = [
    ["Transport reachability", snapshot?.tailscale.online, snapshot?.tailscale.state ?? "Not verified"],
    ["SSH client", snapshot?.sshClient.installed, snapshot?.sshClient.version ?? "Not verified"],
    ["SSH service", snapshot?.sshService.running, snapshot?.sshService.name ?? "Not verified"],
    ["Listener", Boolean(snapshot?.sshPort), snapshot?.sshPort ? `TCP ${snapshot.sshPort}` : "Not verified"],
    ["Configuration syntax", snapshot?.sshConfigValid, snapshot?.sshConfigValid ? "Valid" : "Not verified"],
    ["Firewall scope", Boolean(snapshot?.firewall.ports?.length), snapshot?.firewall.provider ?? "Not verified"]
  ];
  return `
    <article class="panel material">
      <div class="section-heading"><div><p class="eyebrow">NO ELEVATION</p><h2>Verify every layer independently</h2></div><button id="run-verify" class="button primary" ${state.busy ? "disabled" : ""}>Run verify</button></div>
      <div class="verify-list">${checks.map(([label, ok, note]) => `<div><span class="verify-icon ${ok ? "pass" : "neutral"}">${ok ? checkIcon() : dotIcon()}</span><div><strong>${label}</strong><p>${escapeHtml(String(note))}</p></div><b>${ok ? "PASS" : "WAITING"}</b></div>`).join("")}</div>
      <p class="boundary-note">A local Verify proves local state. A real connection test from another host is still required to prove routing, KEX, authentication, and the remote token.</p>
    </article>
  `;
}

function renderRecovery(): string {
  return `
    <div class="recovery-grid">
      <article class="panel material">
        <p class="eyebrow">ROLLBACK JOURNAL</p>
        <h2>Reverse a recorded Apply</h2>
        <p class="muted">Only reversible actions that completed are replayed, in reverse order.</p>
        <label class="field"><span>Journal path</span><input id="journal-path" value="${escapeAttribute(state.report?.journalPath ?? "")}" placeholder="artifacts/apply-….journal.json" /></label>
        <button id="run-rollback" class="button danger-button">Review rollback</button>
      </article>
      <article class="panel recovery-note">
        <div class="dialog-icon warning">${warningIcon()}</div>
        <p class="eyebrow">CONTROL CHANNEL</p>
        <h2>Never restart your only way back in</h2>
        <p>Use delayed actions, a second LAN/console path, and an external Verify target. Tailscale is transport—not proof that SSH is healthy.</p>
      </article>
    </div>
  `;
}

function renderAdvanced(): string {
  const p = state.profile;
  return `
    <article class="panel material">
      <div class="section-heading"><div><p class="eyebrow">PROFILE</p><h2>Common path first, sharp edges second</h2></div><span class="saved-indicator" id="saved-indicator">Changes stay local</span></div>
      <div class="form-grid">
        ${selectField("target-platform", "Target layer", p.target.platform, [["auto", "Auto detect"], ["windows", "Windows"], ["linux", "Linux"], ["macos", "macOS"], ["wsl", "WSL (separate)"]])}
        ${numberField("ssh-port", "SSH port", p.ssh.port, 1, 65535)}
        ${selectField("transport-mode", "Transport", p.transport.mode, [["tailnet", "Tailnet (recommended)"], ["lan", "LAN"], ["custom", "Custom"], ["none", "None"]])}
        ${selectField("exposure-mode", "Exposure", p.exposure.mode, [["tailnet", "Tailnet only"], ["lan", "LAN"], ["custom", "Custom CIDRs"], ["none", "No firewall opening"]])}
        ${selectField("download-strategy", "Download source", p.download.strategy, [["official", "Official release"], ["package-manager", "System package manager"], ["mirror", "Explicit trusted mirror"], ["proxy", "Explicit proxy"], ["offline", "Offline bundle"], ["cache", "Verified cache"]])}
        ${numberField("download-retries", "Retries", p.download.retries, 0, 10)}
      </div>
      <details class="advanced-details">
        <summary>Safety and authentication</summary>
        <div class="detail-content">
          <label class="field full"><span>Controller public keys (one per line)</span><textarea id="public-keys" rows="5" placeholder="ssh-ed25519 AAAA… controller">${escapeHtml(p.ssh.publicKeys.join("\n"))}</textarea><small>Private keys and tokens are rejected by the core.</small></label>
          <label class="check-row"><input id="prevent-self-cut" type="checkbox" ${p.safety.preventSelfCut ? "checked" : ""} /><span>Block active-channel self-cut</span></label>
          <label class="check-row"><input id="auto-rollback" type="checkbox" ${p.safety.autoRollback ? "checked" : ""} /><span>Rollback completed reversible actions after partial failure</span></label>
          <label class="check-row"><input id="password-auth" type="checkbox" ${p.ssh.passwordAuthentication ? "checked" : ""} /><span>Allow password authentication (not recommended)</span></label>
        </div>
      </details>
      <div class="panel-footer"><span>Profiles contain intent, never private keys, tokens, real device logs, or credentials.</span><button id="save-profile" class="button primary">Use this profile</button></div>
    </article>
  `;
}

function bindPlanEvents(): void {
  document.querySelector("#refresh-plan")?.addEventListener("click", () => runStage("plan"));
  document.querySelector("#open-apply")?.addEventListener("click", openApplyDialog);
}

function bindRecoveryEvents(): void {
  document.querySelector("#run-rollback")?.addEventListener("click", async () => {
    const path = document.querySelector<HTMLInputElement>("#journal-path")!.value.trim();
    if (!path) {
      announce("Enter a journal path first.");
      return;
    }
    if (!window.go?.main?.App) {
      announce("Rollback is only available in the native app.");
      return;
    }
    state.busy = true;
    try {
      state.report = await window.go.main.App.Rollback(path);
      state.activeView = "progress";
      syncNavigation();
    } catch (error) {
      announce(errorMessage(error));
    } finally {
      state.busy = false;
      render();
    }
  });
}

function bindAdvancedEvents(): void {
  document.querySelector("#save-profile")?.addEventListener("click", () => {
    state.profile.target.platform = valueOf("target-platform");
    state.profile.ssh.port = Number(valueOf("ssh-port"));
    state.profile.transport.mode = valueOf("transport-mode");
    state.profile.exposure.mode = valueOf("exposure-mode");
    state.profile.download.strategy = valueOf("download-strategy");
    state.profile.download.retries = Number(valueOf("download-retries"));
    state.profile.ssh.publicKeys = valueOf("public-keys").split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
    state.profile.safety.preventSelfCut = checked("prevent-self-cut");
    state.profile.safety.autoRollback = checked("auto-rollback");
    state.profile.ssh.passwordAuthentication = checked("password-auth");
    setText("#saved-indicator", "Profile active");
    announce("Profile updated locally.");
  });
}

async function runStage(stage: Stage, confirmed = false, scheduleRisky = false, externalVerify = ""): Promise<void> {
  if (state.busy) return;
  state.busy = true;
  state.progress = stage === "apply" ? [] : state.progress;
  announce(`${stage} started.`);
  refreshBusyControls();
  const request: DesktopRequest = { stage, profile: state.profile, confirmed, allowSelfCut: false, scheduleRisky, externalVerify };
  try {
    state.report = window.go?.main?.App ? await window.go.main.App.Run(request) : await mockRun(request);
    announce(`${stage} ${state.report.success ? "completed" : "needs attention"}.`);
    if (stage === "apply") {
      state.activeView = "progress";
      syncNavigation();
    } else if (stage === "plan") {
      state.activeView = "plan";
      syncNavigation();
    } else if (stage === "verify") {
      state.activeView = "verify";
      syncNavigation();
    }
  } catch (error) {
    announce(errorMessage(error));
  } finally {
    state.busy = false;
    refreshBusyControls();
    document.querySelector<HTMLButtonElement>("#export-report")!.disabled = !state.report;
    render(true);
  }
}

function openApplyDialog(): void {
  const actions = state.report?.plan?.actions ?? [];
  if (!actions.length) return;
  setText("#apply-summary", `${actions.length} action${actions.length === 1 ? "" : "s"} will change this host. Review each layer.`);
  document.querySelector("#apply-actions")!.innerHTML = actions.map((action) => `<div><span class="risk ${action.risk}">${action.risk}</span><div><strong>${escapeHtml(action.summary)}</strong><p>${escapeHtml(action.reason)}</p></div></div>`).join("");
  const ack = document.querySelector<HTMLInputElement>("#apply-ack")!;
  ack.checked = false;
  document.querySelector<HTMLButtonElement>("#confirm-apply")!.disabled = true;
  const dialog = document.querySelector<HTMLDialogElement>("#apply-dialog")!;
  dialog.showModal();
  requestAnimationFrame(() => ack.focus());
}

async function exportReport(): Promise<void> {
  if (!state.report) return;
  if (window.go?.main?.App) {
    const path = await window.go.main.App.ExportReport(state.report);
    announce(path ? `Report exported to ${path}` : "Export cancelled.");
    return;
  }
  const blob = new Blob([JSON.stringify(state.report, null, 2)], { type: "application/json" });
  const link = document.createElement("a");
  link.href = URL.createObjectURL(blob);
  link.download = `${state.report.id}.report.json`;
  link.click();
  URL.revokeObjectURL(link.href);
}

async function mockRun(request: DesktopRequest): Promise<Report> {
  await delay(240);
  const snapshot: Snapshot = {
    platform: request.profile.target.platform === "auto" ? "windows" : request.profile.target.platform,
    arch: "amd64",
    hostname: "DEMO-HOST",
    isAdministrator: false,
    sessionTransport: "local",
    packageManager: "winget",
    sshClient: { installed: true, version: "OpenSSH_9.x" },
    sshServer: { installed: true },
    sshService: { name: "sshd", installed: true, running: false, startPolicy: "Manual" },
    sshPort: 22,
    sshConfigValid: true,
    firewall: { provider: "windows-firewall", ports: [], scopes: [] },
    tailscale: { installed: true, online: true, ip: "100.64.0.10", state: "Running" },
    network: { githubDns: true, tailscaleDns: true, proxySet: false }
  };
  const actions: PlanAction[] = [
    { id: "enable-sshd", operation: "enable_sshd", layer: "ssh-service", risk: "medium", summary: "Enable and start the SSH service", reason: "The service is installed but not running.", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true },
    { id: "configure-firewall", operation: "configure_firewall", layer: "firewall", risk: "high", summary: `Allow TCP ${request.profile.ssh.port} from Tailnet only`, reason: "No port-and-scope-aware rule matches.", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true }
  ];
  if (request.stage === "apply") {
    for (const action of actions) {
      state.progress.push({ kind: "started", actionId: action.id, message: action.summary });
      render();
      await delay(260);
      state.progress.push({ kind: "completed", actionId: action.id, message: "completed" });
    }
    return { id: `apply-${Date.now()}`, stage: "apply", success: true, exitCode: 0, profileName: request.profile.name, snapshot, plan: { noChanges: false, highestRisk: "high", selfCutDetected: false, actions }, results: actions.map((action) => ({ actionId: action.id, status: "completed" })), journalPath: "artifacts/demo.journal.json" };
  }
  const verified = request.stage === "verify";
  if (verified) {
    snapshot.sshService.running = true;
    snapshot.firewall.ports = [request.profile.ssh.port];
  }
  return { id: `${request.stage}-${Date.now()}`, stage: request.stage, success: true, exitCode: 0, profileName: request.profile.name, snapshot, plan: { noChanges: verified, highestRisk: "high", selfCutDetected: false, actions: verified ? [] : actions } };
}

function actionRow(action: PlanAction): string {
  return `<div class="action-row"><span class="risk ${action.risk}">${action.risk}</span><div class="action-copy"><div><strong>${escapeHtml(action.summary)}</strong>${action.selfCutRisk ? `<span class="self-cut">self-cut risk</span>` : ""}</div><p>${escapeHtml(action.reason)}</p><small>${escapeHtml(action.layer)} · ${action.reversible ? "rollback covered" : "manual recovery"}${action.requiresElevation ? " · elevation" : ""}</small></div>${chevronIcon()}</div>`;
}

function statusCard(title: string, value: string, ok: boolean, note: string, icon: string): string {
  return `<article class="status-card"><div class="status-icon">${icon}</div><div><p>${escapeHtml(title)}</p><strong>${escapeHtml(value)}</strong><small>${escapeHtml(note)}</small></div><span class="state-dot ${ok ? "ok" : "neutral"}" aria-label="${ok ? "healthy" : "not verified"}"></span></article>`;
}

function renderWarnings(warnings?: string[]): string {
  if (!warnings?.length) return "";
  return `<aside class="warning-banner" role="status">${warningIcon()}<div><strong>Attention</strong>${warnings.map((warning) => `<p>${escapeHtml(warning)}</p>`).join("")}</div></aside>`;
}

function metric(label: string, value: string): string {
  return `<div><span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong></div>`;
}

function emptyState(title: string, body: string): string {
  return `<div class="empty-state">${planIcon()}<strong>${escapeHtml(title)}</strong><p>${escapeHtml(body)}</p></div>`;
}

function selectField(id: string, label: string, value: string, options: Array<[string, string]>): string {
  return `<label class="field"><span>${label}</span><select id="${id}">${options.map(([key, text]) => `<option value="${key}" ${key === value ? "selected" : ""}>${text}</option>`).join("")}</select></label>`;
}

function numberField(id: string, label: string, value: number, min: number, max: number): string {
  return `<label class="field"><span>${label}</span><input id="${id}" type="number" value="${value}" min="${min}" max="${max}" /></label>`;
}

function healthScore(snapshot: Snapshot): string {
  const signals = [snapshot.sshClient.installed, snapshot.sshServer.installed, snapshot.sshService.running, snapshot.sshConfigValid, snapshot.tailscale.online];
  return `${Math.round(signals.filter(Boolean).length / signals.length * 100)}%`;
}

function healthClass(snapshot: Snapshot): string {
  return snapshot.sshService.running && snapshot.sshConfigValid ? "healthy" : "attention";
}

function healthLabel(snapshot: Snapshot): string {
  return snapshot.sshService.running && snapshot.sshConfigValid ? "Locally ready" : "Needs a plan";
}

function animateFromCurrent(element: HTMLElement): void {
  if (matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  const current = getComputedStyle(element);
  element.getAnimations().forEach((animation) => animation.cancel());
  element.animate(
    [
      { opacity: Number(current.opacity) || 0.65, transform: current.transform === "none" ? "translateY(6px)" : current.transform },
      { opacity: 1, transform: "translateY(0)" }
    ],
    { duration: 280, easing: "cubic-bezier(.2,.8,.2,1)" }
  );
}

function toggleTheme(): void {
  const current = document.documentElement.dataset.theme;
  document.documentElement.dataset.theme = current === "dark" ? "light" : "dark";
}

function refreshBusyControls(): void {
  document.querySelector<HTMLButtonElement>("#run-check")!.disabled = state.busy;
  document.querySelector<HTMLButtonElement>("#run-check")!.textContent = state.busy ? "Working…" : "Run check";
}

function syncNavigation(): void {
  document.querySelectorAll<HTMLElement>(".nav-item").forEach((item) => {
    const active = item.dataset.view === state.activeView;
    item.classList.toggle("active", active);
    item.setAttribute("aria-current", active ? "page" : "false");
  });
  setText("#view-title", titles[state.activeView] ?? "SSH Launchpad");
}

function announce(message: string): void {
  setText("#announcer", message);
}

function setText(selector: string, value: string): void {
  const element = document.querySelector(selector);
  if (element) element.textContent = value;
}

function valueOf(id: string): string {
  return (document.querySelector<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>(`#${id}`)?.value ?? "").trim();
}

function checked(id: string): boolean {
  return document.querySelector<HTMLInputElement>(`#${id}`)?.checked ?? false;
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" })[character] ?? character);
}

function escapeAttribute(value: string): string {
  return escapeHtml(value);
}

function svg(path: string): string {
  return `<svg viewBox="0 0 24 24" aria-hidden="true">${path}</svg>`;
}

function statusIcon(): string { return svg('<path d="M4 13h4l2-7 4 12 2-5h4"/><path d="M3 5h18v14H3z"/>'); }
function planIcon(): string { return svg('<path d="M5 4h14v16H5z"/><path d="M8 8h8M8 12h8M8 16h5"/>'); }
function progressIcon(): string { return svg('<circle cx="12" cy="12" r="8"/><path d="M12 8v5l3 2"/>'); }
function verifyIcon(): string { return svg('<path d="M12 3 5 6v5c0 5 3 8 7 10 4-2 7-5 7-10V6z"/><path d="m9 12 2 2 4-5"/>'); }
function recoveryIcon(): string { return svg('<path d="M4 8v5h5"/><path d="M5.5 16a8 8 0 1 0 0-8L4 10"/>'); }
function advancedIcon(): string { return svg('<path d="M4 7h10M18 7h2M4 17h2M10 17h10M14 4v6M6 14v6"/>'); }
function themeIcon(): string { return svg('<path d="M20 15.5A8 8 0 0 1 8.5 4 8 8 0 1 0 20 15.5z"/>'); }
function warningIcon(): string { return svg('<path d="M12 3 2.5 20h19z"/><path d="M12 9v5m0 3h.01"/>'); }
function platformIcon(): string { return svg('<rect x="4" y="5" width="16" height="11" rx="2"/><path d="M8 20h8M12 16v4"/>'); }
function transportIcon(): string { return svg('<path d="M5 12a7 7 0 0 1 14 0M8 15a4 4 0 0 1 8 0"/><circle cx="12" cy="19" r="1"/>'); }
function serviceIcon(): string { return svg('<path d="M6 4h12v16H6z"/><path d="M9 8h6M9 12h6M9 16h3"/>'); }
function firewallIcon(): string { return svg('<path d="M4 5h16v14H4zM4 10h16M8 5v5m8-5v5m-8 5v4m8-4v4m-4-9v5"/>'); }
function networkIcon(): string { return svg('<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18"/>'); }
function configIcon(): string { return svg('<path d="M6 3h9l3 3v15H6z"/><path d="M14 3v4h4M9 12h6m-6 4h6"/>'); }
function chevronIcon(): string { return svg('<path d="m9 6 6 6-6 6"/>'); }
function checkIcon(): string { return svg('<path d="m5 12 4 4L19 6"/>'); }
function dotIcon(): string { return svg('<circle cx="12" cy="12" r="2"/>'); }

void initialise();
