import { delay, setText } from "./browser-utils";
import type { ControllerContext } from "./controller-context";
import type { DesktopRequest, ElevatedJob, Report, Stage } from "./types";
import { checkIssues, confirmAckKey, renderConfirmActions, type WizardMode } from "./views";

export function createWizard(context: ControllerContext) {
  const { state, backend, t, renderPage, feedback } = context;
  const { showToast, friendlyError, friendlyReportError, confirmDialog } = feedback;
  let keyInputTimer: ReturnType<typeof setTimeout> | undefined;

  // Validate pasted keys after input settles, rather than waiting for blur.
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
    try {
      await backend.validatePublicKey(key);
      return true;
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
      profile: structuredClone(state.profile),
      planDigest: plan?.digest ?? "",
      confirmed: true,
      allowSelfCut: false,
      scheduleRisky: false,
      externalVerify: "",
      planNoChanges: plan?.noChanges ?? false,
      planNeedsElevation: (plan?.actions ?? []).some((action) => action.mutating && action.requiresElevation)
    };
    try {
      state.activeJob = await backend.beginApply(request, {
        language: () => state.language,
        progress: (event) => {
          state.installState = "running";
          state.progress.push(event);
          renderPage();
        }
      });
      if (["completed", "failed", "cancelled"].includes(state.activeJob.state)) {
        finishElevatedJob(state.activeJob);
      } else {
        await pollElevatedJob(state.activeJob.id);
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
      if (Date.now() > deadline && !state.installError) {
        // A UI deadline is not a process cancellation. Keep the job attached
        // and block conflicting navigation until the helper reports terminal.
        state.installError = t("installTimeout");
        showToast(state.installError);
      }
      let job: ElevatedJob;
      try {
        job = await backend.jobStatus(id);
      } catch {
        state.installError = t("installTimeout");
        await delay(2000);
        continue;
      }
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
        await backend.dismissJob(id);
        return;
      }
      await delay(500);
    }
  }

  function finishElevatedJob(job: ElevatedJob): void {
    if (job.report) {
      state.report = job.report;
      if (job.report.journalPath) state.mutationReport = job.report;
    }
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
    return backend.run(request);
  }

  async function rollbackLast(): Promise<void> {
    if (state.busy || !state.mutationReport?.journalPath || backend.kind !== "desktop") return;
    if (!(await confirmDialog(t("rollbackLast"), t("rollbackConfirmBody")))) return;
    state.busy = true;
    renderPage();
    try {
      const report = await backend.rollback(state.mutationReport.journalPath);
      state.report = report;
      state.mutationReport = report;
      if (report.success) state.planReport = undefined;
      state.installState = report.success ? "idle" : "failed";
      state.installError = report.success ? "" : (report.error || t("recoveryFailed"));
      if (state.view === "wizard") state.step = report.success ? 0 : 1;
      showToast(report.success ? t("recoverySucceeded") : (report.error || t("recoveryFailed")));
    } catch (error) {
      showToast(friendlyError(error));
    } finally {
      state.busy = false;
      renderPage();
    }
  }

  function goHome(): void {
    if (state.busy || state.installState === "waiting-for-permission" || state.installState === "running") {
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

  return {
    bindPublicKeyInput, startWizard, onCheckContinue, runPlanStage, setNetworkMode,
    runCheck, publicKeyIsValid, openInstallDialog, beginSafeInstall, runVerify,
    runStage, rollbackLast, goHome
  };
}

export type WizardActions = ReturnType<typeof createWizard>;
