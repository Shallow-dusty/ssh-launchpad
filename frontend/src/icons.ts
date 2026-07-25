function svg(content: string): string {
  return `<svg viewBox="0 0 24 24" aria-hidden="true">${content}</svg>`;
}

export function launchIcon(): string { return svg('<path d="M5 8.5 12 4l7 4.5v7L12 20l-7-4.5z"/><path d="m9 11 2 2-2 2m4 0h3"/>'); }
export function themeIcon(): string { return svg('<path d="M12 3a9 9 0 1 0 9 9c-5 2-11-4-9-9z"/>'); }
export function devicesIcon(): string { return svg('<rect x="2.5" y="5" width="11" height="8" rx="1.5"/><path d="M6 17h4m-2-4v4"/><rect x="15.5" y="8" width="6" height="10" rx="1.3"/>'); }
export function arrowIcon(): string { return svg('<path d="M5 12h14m-5-5 5 5-5 5"/>'); }
export function screenIcon(): string { return svg('<rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8m-4-4v4"/>'); }
export function repairIcon(): string { return svg('<path d="M14.5 6.5a4 4 0 0 0-5-5L12 4 9 7 6.5 4.5a4 4 0 0 0 5 5L19 17a1.4 1.4 0 0 1-2 2z"/>'); }
export function slidersIcon(): string { return svg('<path d="M4 7h10m4 0h2M4 17h2m4 0h10M14 4v6M6 14v6"/>'); }
export function lockIcon(): string { return svg('<rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>'); }
export function backIcon(): string { return svg('<path d="m15 18-6-6 6-6"/>'); }
export function searchIcon(): string { return svg('<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>'); }
export function checkIcon(): string { return svg('<path d="m5 12 4 4L19 6"/>'); }
export function shieldIcon(): string { return svg('<path d="M12 3 20 6v5c0 5-3.4 8.2-8 10-4.6-1.8-8-5-8-10V6z"/><path d="m8.5 12 2.2 2.2 4.8-5"/>'); }
export function infoIcon(): string { return svg('<circle cx="12" cy="12" r="9"/><path d="M12 11v5m0-8h.01"/>'); }
export function warningIcon(): string { return svg('<path d="m12 3 10 18H2z"/><path d="M12 9v5m0 3h.01"/>'); }
export function closeIcon(): string { return svg('<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6m0-6-6 6"/>'); }
export function computerIcon(): string { return screenIcon(); }
export function userIcon(): string { return svg('<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>'); }
export function networkIcon(): string { return svg('<circle cx="12" cy="12" r="2"/><path d="M5.6 18.4a9 9 0 0 1 0-12.8m12.8 0a9 9 0 0 1 0 12.8M8.5 15.5a5 5 0 0 1 0-7m7 0a5 5 0 0 1 0 7"/>'); }
export function powerIcon(): string { return svg('<path d="M12 2v10m6.4-6.4a9 9 0 1 1-12.8 0"/>'); }
export function packageIcon(): string { return svg('<path d="m4 7 8-4 8 4v10l-8 4-8-4z"/><path d="m4 7 8 4 8-4m-8 4v10"/>'); }
export function doorIcon(): string { return svg('<path d="M5 21V4l12-2v19M5 21h14"/><path d="M13 12h.01"/>'); }
export function keyIcon(): string { return svg('<circle cx="8" cy="15" r="4"/><path d="m11 12 8-8m-3 3 3 3"/>'); }
export function copyIcon(): string { return svg('<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>'); }
export function uploadIcon(): string { return svg('<path d="M12 16V4m-5 5 5-5 5 5M4 20h16"/>'); }
export function downloadIcon(): string { return svg('<path d="M12 4v12m-5-5 5 5 5-5M4 20h16"/>'); }
