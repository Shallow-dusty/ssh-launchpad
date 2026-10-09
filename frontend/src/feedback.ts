import { announce, setText } from "./browser-utils";
import type { AppState } from "./state";
import { ReportError } from "./model-utils";
import type { Report } from "./types";
import type { Translate } from "./views";

export function createFeedback(state: AppState, t: Translate) {
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

  // Prefer the specific reason over coarse exit codes or English prose.
  function friendlyReportError(raw: string, report?: Report): string {
    switch (report?.reasonCode) {
      case "confirmation_required": return t("errorConfirmationRequired");
      case "plan_changed": return t("errorPlanChanged");
      case "mutation_busy": return t("errorMutationBusy");
      case "elevation_required": return t("errorElevationRequired");
      case "download_failed": return t("errorDownload");
      case "self_cut_blocked": return t("errorSelfCut");
    }
    // Older binaries have no reasonCode; retain their exit-code fallback.
    if (report?.reasonCode) return raw || t("errorGeneric");
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
    if (error instanceof ReportError) return friendlyReportError(error.message, error.report);
    const raw = error instanceof Error ? error.message : String(error ?? "");
    return raw || t("errorGeneric");
  }

  async function copyConnectionCommand(handoff = false): Promise<void> {
    const command = document.querySelector<HTMLElement>(".command-box code")?.textContent ?? "";
    const snapshot = state.verifyReport?.snapshot;
    const code = handoff
      ? [`SSH Launchpad`, `${t("factHost")}: ${snapshot?.hostname ?? "—"}`, command, t("localVerifyOnly"), t("firstConnect2")].join("\n")
      : command;
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

  return { confirmDialog, showToast, friendlyReportError, friendlyError, copyConnectionCommand };
}

export type Feedback = ReturnType<typeof createFeedback>;
