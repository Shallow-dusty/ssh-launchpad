import { checked, setText, valueOf } from "./browser-utils";
import type { ControllerContext } from "./controller-context";
import type { Profile, Stage } from "./types";
import type { WizardActions } from "./wizard";

export function createAdvanced(context: ControllerContext, runStage: WizardActions["runStage"]) {
  const { state, backend, t, renderPage, feedback } = context;
  const { showToast, friendlyError } = feedback;

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
    onInput("target-platform", (value) => { state.profile.target.platform = value as Profile["target"]["platform"]; });
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
    const path = await backend.exportReport(state.report);
    if (path) showToast(path);
  }

  async function checkForUpdate(): Promise<void> {
    try {
      const info = await backend.checkForUpdate();
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

  return { runAdvancedStage, bindAdvancedAutoApply, exportReport, checkForUpdate };
}
