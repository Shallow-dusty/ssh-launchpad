import type { Backend } from "./backend";
import type { Feedback } from "./feedback";
import type { AppState } from "./state";
import type { Translate } from "./views";

export interface ControllerContext {
  state: AppState;
  backend: Backend;
  t: Translate;
  renderPage(): void;
  feedback: Feedback;
}
