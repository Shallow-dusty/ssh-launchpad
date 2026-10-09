import type { ControllerContext } from "./controller-context";
import { defaultProfile, normalizeProfile } from "./profile";
import type { PersonalCard, Profile, PublicKeyInfo } from "./types";
import type { WizardActions } from "./wizard";

export function createTransfers(context: ControllerContext, wizard: WizardActions) {
  const { state, backend, t, renderPage, feedback } = context;
  const { showToast, friendlyError } = feedback;
  const { publicKeyIsValid, runPlanStage, startWizard } = wizard;

  async function importPublicKey(): Promise<void> {
    try {
      const key = await backend.importPublicKey();
      if (key?.publicKey) selectKey(key);
    } catch (error) {
      showToast(friendlyError(error));
    }
  }

  async function generatePublicKey(): Promise<void> {
    try {
      const key = await backend.generateControllerKey("ssh-launchpad-controller");
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
    const path = await backend.exportPairingFile(key);
    if (path) showToast(path);
    else if (backend.kind === "preview") showToast(t("exportPairing"));
  }

  async function importProfile(): Promise<void> {
    try {
      const profile = await backend.importProfile();
      if (profile?.schemaVersion) {
        state.profile = normalizeProfile(profile);
        state.selectedKey = state.profile.ssh.publicKeys[0] ? { label: t("profileImported"), path: "", publicKey: state.profile.ssh.publicKeys[0], generated: false } : undefined;
        showToast(t("profileImported"));
        renderPage();
      }
    } catch (error) {
      showToast(friendlyError(error));
    }
  }

  async function exportProfile(): Promise<void> {
    if (await backend.exportProfile(state.profile)) showToast(t("profileExported"));
  }

  async function importPersonalCard(): Promise<void> {
    state.cardImportError = "";
    try {
      const card = await backend.importPersonalCard();
      // No card (preview picker) or an empty desktop result means no import yet.
      if (!card?.schemaVersion) return;
      await applyPersonalCard(card);
      showToast(t("cardImported"));
    } catch (error) {
      state.cardImportError = friendlyError(error);
      renderPage();
    }
  }

  async function exportPersonalCard(): Promise<void> {
    const card = buildPersonalCard();
    const error = await validatePersonalCardClient(card);
    if (error) {
      showToast(error);
      return;
    }
    if (await backend.exportPersonalCard(card)) showToast(t("cardExported"));
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
    const publicKeys = card.ssh.publicKeys ?? [];
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
        publicKeys: [...publicKeys]
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
    const firstKey = publicKeys[0]!;
    state.selectedKey = { label: card.controllerName?.trim() || t("cardTitle"), path: "", publicKey: firstKey, generated: false };
    for (const publicKey of publicKeys) {
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

  return {
    importPublicKey, generatePublicKey, exportPairing, importProfile, exportProfile,
    importPersonalCard, exportPersonalCard, importProfileFromBrowser,
    importPersonalCardFromBrowser, importKeyFromBrowser
  };
}
