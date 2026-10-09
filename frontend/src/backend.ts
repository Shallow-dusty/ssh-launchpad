import { delay, downloadText } from "./browser-utils";
import { translate, type Language } from "./i18n";
import { mockActions, mockPublicKey, mockRun } from "./mock-backend";
import { isNewerVersion, profileToYAML, redactReport, ReportError } from "./model-utils";
import { defaultProfile, normalizeProfile } from "./profile";
import type { DesktopBridge, DesktopRequest, ElevatedJob, PersonalCard, Profile, PublicKeyInfo, Report, UpdateInfo } from "./types";
import { APP_VERSION } from "./version";

export type JobEvent = NonNullable<ElevatedJob["events"]>[number];
export interface ApplyObserver {
  language(): Language;
  progress(event: JobEvent): void;
}

// Bind one backend at startup. A desktop failure must never fall through to
// simulated success; the preview only exists when no Wails bridge is present.
export interface Backend {
  readonly kind: "desktop" | "preview";
  defaultProfile(): Promise<Profile>;
  discoverPublicKeys(): Promise<PublicKeyInfo[]>;
  validatePublicKey(value: string): Promise<void>;
  run(request: DesktopRequest): Promise<Report>;
  beginApply(request: DesktopRequest, observer: ApplyObserver): Promise<ElevatedJob>;
  jobStatus(id: string): Promise<ElevatedJob>;
  dismissJob(id: string): Promise<void>;
  rollback(path: string): Promise<Report>;
  importPublicKey(): Promise<PublicKeyInfo | null>;
  generateControllerKey(label: string): Promise<PublicKeyInfo>;
  exportPairingFile(key: string): Promise<string | null>;
  importProfile(): Promise<Profile | null>;
  exportProfile(profile: Profile): Promise<boolean>;
  importPersonalCard(): Promise<PersonalCard | null>;
  exportPersonalCard(card: PersonalCard): Promise<boolean>;
  exportReport(report: Report): Promise<string | null>;
  checkForUpdate(): Promise<UpdateInfo>;
}

export function createBackend(app: DesktopBridge | undefined = window.go?.main?.App): Backend {
  if (app) {
    return {
      kind: "desktop",
      defaultProfile: async () => normalizeProfile(await app.DefaultProfile()),
      discoverPublicKeys: async () => await app.DiscoverPublicKeys() ?? [],
      validatePublicKey: (value) => app.ValidatePublicKey(value),
      run: async (request) => {
        const report = await app.Run(request);
        // Verify drift is an inspectable result with remaining actions. Other
        // failed stages stay errors, retaining their machine-readable report.
        if (!report.success && !(request.stage === "verify" && report.plan)) throw new ReportError(report);
        return report;
      },
      beginApply: (request) => app.BeginElevatedApply(request),
      jobStatus: (id) => app.ElevatedApplyStatus(id),
      dismissJob: (id) => app.DismissElevatedJob(id),
      rollback: (path) => app.Rollback(path),
      importPublicKey: () => app.ImportPublicKey(),
      generateControllerKey: (label) => app.GenerateControllerKey(label),
      exportPairingFile: (key) => app.ExportPairingFile(key),
      importProfile: async () => {
        const profile = await app.ImportProfile();
        return profile.schemaVersion ? normalizeProfile(profile) : null;
      },
      exportProfile: async (profile) => Boolean(await app.ExportProfile(profile)),
      importPersonalCard: () => app.ImportPersonalCard(),
      exportPersonalCard: async (card) => Boolean(await app.ExportPersonalCard(card)),
      exportReport: (report) => app.ExportReport(report),
      checkForUpdate: () => app.CheckForUpdate()
    };
  }
  return createPreviewBackend();
}

function chooseFile(selector: string): null {
  document.querySelector<HTMLInputElement>(selector)!.click();
  return null;
}

function createPreviewBackend(): Backend {
  return {
    kind: "preview",
    defaultProfile: async () => structuredClone(defaultProfile),
    discoverPublicKeys: async () => new URLSearchParams(location.search).get("mock") === "no-public-key" ? [] : [mockPublicKey()],
    validatePublicKey: async (key) => {
      // Preview-only shape check. Desktop validation always uses Go's SSH
      // parser; this is not an alternate authority for real installation.
      const [algorithm = "", payload = ""] = key.trim().split(/\s+/);
      if (!payload || !/^(ssh-(ed25519|rsa)|ecdsa-sha2-nistp(256|384|521)|sk-ssh-ed25519@openssh\.com|sk-ecdsa-sha2-nistp256@openssh\.com)$/.test(algorithm)
        || atob(payload).length <= 16) throw new Error("Invalid preview public key");
    },
    run: mockRun,
    beginApply: mockApply,
    jobStatus: async () => { throw new Error("Preview jobs complete in process and cannot be polled"); },
    dismissJob: async () => {},
    rollback: async () => { throw new Error("System rollback is unavailable in preview mode"); },
    importPublicKey: async () => chooseFile("#key-file"),
    generateControllerKey: async () => mockPublicKey(true),
    exportPairingFile: async (key) => {
      downloadText("ssh-launchpad-controller.pub", `${key}\n`, "text/plain;charset=utf-8");
      return null;
    },
    importProfile: async () => chooseFile("#profile-file"),
    exportProfile: async (profile) => {
      downloadText(`${profile.name}.ssh-launchpad.yaml`, profileToYAML(profile), "text/yaml;charset=utf-8");
      return true;
    },
    importPersonalCard: async () => chooseFile("#card-file"),
    exportPersonalCard: async (card) => {
      const name = card.displayName.trim().replace(/[<>:"/\\|?*\x00-\x1f]/g, "-").replace(/[. ]+$/g, "") || "ssh-launchpad-setup";
      downloadText(`${name}.sshlaunchpad-card`, `${JSON.stringify(card, null, 2)}\n`, "application/json;charset=utf-8");
      return true;
    },
    exportReport: async (report) => {
      downloadText(`${report.id}.report.json`, `${JSON.stringify(redactReport(report), null, 2)}\n`, "application/json;charset=utf-8");
      return null;
    },
    checkForUpdate: async () => {
      const response = await fetch("https://api.github.com/repos/Shallow-dusty/ssh-launchpad/releases/latest");
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const value = await response.json() as { tag_name: string; html_url: string };
      const latest = value.tag_name.replace(/^v/, "");
      return { currentVersion: APP_VERSION, latestVersion: latest, available: isNewerVersion(latest, APP_VERSION), url: value.html_url, channel: "stable" };
    }
  };
}

async function mockApply(request: DesktopRequest, observer: ApplyObserver): Promise<ElevatedJob> {
  await delay(250);
  const mode = new URLSearchParams(location.search).get("mock");
  const attempt = Number(sessionStorage.getItem("ssh-launchpad-mock-attempt") ?? "0") + 1;
  sessionStorage.setItem("ssh-launchpad-mock-attempt", String(attempt));
  if (mode === "uac-cancel" && attempt === 1) return { id: "mock", state: "cancelled", error: translate(observer.language(), "installCancelled") };
  if (mode === "fail" && attempt === 1) return { id: "mock", state: "failed", error: observer.language() === "zh-CN" ? "模拟：网络中断，校验失败，电脑没有继续改动。" : "Simulated network interruption; verification failed and later changes stopped." };
  for (const action of mockActions(request.profile.ssh.port)) {
    observer.progress({ timestamp: new Date().toISOString(), stage: "apply", kind: "started", actionId: action.id, message: action.summary });
    await delay(140);
    observer.progress({ timestamp: new Date().toISOString(), stage: "apply", kind: "completed", actionId: action.id, message: "completed" });
  }
  localStorage.setItem("ssh-launchpad-demo-ready", "true");
  const report = await mockRun({ ...request, stage: "apply" });
  report.success = true;
  report.exitCode = 0;
  report.results = mockActions(request.profile.ssh.port).map((action) => ({
    actionId: action.id, status: "completed", started: report.started, finished: report.finished
  }));
  return { id: "mock", state: "completed", report };
}
