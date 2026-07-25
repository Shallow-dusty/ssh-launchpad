export function valueOf(id: string): string {
  return document.querySelector<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>(`#${id}`)?.value ?? "";
}

export function checked(id: string): boolean {
  return document.querySelector<HTMLInputElement>(`#${id}`)?.checked ?? false;
}

export function setText(selector: string, text: string): void {
  const node = document.querySelector<HTMLElement>(selector);
  if (node) node.textContent = text;
}

export function announce(text: string): void {
  setText("#announcer", text);
}

export function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function escapeHtml(value: unknown): string {
  return String(value ?? "").replace(/[&<>"']/g, (character) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#039;"
  })[character]!);
}

export function escapeAttribute(value: unknown): string {
  return escapeHtml(value);
}

export function downloadText(name: string, text: string, type: string): void {
  const url = URL.createObjectURL(new Blob([text], { type }));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}

export function animateFromCurrent(element: HTMLElement): void {
  if (matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  element.getAnimations().forEach((animation) => animation.cancel());
  element.animate(
    [{ opacity: .65, transform: "translateY(5px)" }, { opacity: 1, transform: "translateY(0)" }],
    { duration: 220, easing: "cubic-bezier(.2,.8,.2,1)" }
  );
}
