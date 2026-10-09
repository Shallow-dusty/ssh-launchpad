import { detectLanguage } from "./i18n";
import { defaultProfile } from "./profile";
import type { ViewState } from "./views";

export type View = "home" | "wizard" | "advanced";
export interface AppState extends ViewState {
  view: View;
  backend: boolean;
  toast: string;
}

export function createState(backend: boolean): AppState {
  return {
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
    backend,
    detectedKeys: [],
    progress: [],
    planError: "",
    installState: "idle",
    installError: "",
    checkError: "",
    cardImportError: "",
    verifyError: "",
    toast: "",
    showNetwork: false,
    showKey: false,
    keyAttempted: false
  };
}
