import type { Profile, Report } from "./types";

export function isNewerVersion(candidate: string, current: string): boolean {
  const parse = (value: string) =>
    value
      .replace(/^v/i, "")
      .split(".")
      .slice(0, 3)
      .map((part) => Number(part.split("-")[0] ?? 0) || 0);
  const left = parse(candidate);
  const right = parse(current);
  for (let index = 0; index < 3; index += 1) {
    const difference = (left[index] ?? 0) - (right[index] ?? 0);
    if (difference !== 0) return difference > 0;
  }
  return false;
}

export function profileToYAML(profile: Profile): string {
  const scalar = (value: unknown) => JSON.stringify(value);
  return `schemaVersion: ${profile.schemaVersion}\nname: ${scalar(profile.name)}\ntarget:\n  platform: ${profile.target.platform}\nssh:\n  enabled: ${profile.ssh.enabled}\n  port: ${profile.ssh.port}\n  publicKeys:\n${profile.ssh.publicKeys.map((key) => `    - ${scalar(key)}`).join("\n")}\n  passwordAuthentication: ${profile.ssh.passwordAuthentication}\ntransport:\n  mode: ${profile.transport.mode}\n  install: ${profile.transport.install}\nexposure:\n  mode: ${profile.exposure.mode}\n  customCidrs: []\ndownload:\n  strategy: ${profile.download.strategy}\n  mirrorBaseUrl: ${scalar(profile.download.mirrorBaseUrl)}\n  proxyUrl: ${scalar(profile.download.proxyUrl)}\n  offlineBundle: ${scalar(profile.download.offlineBundle)}\n  cacheDir: ${scalar(profile.download.cacheDir)}\n  retries: ${profile.download.retries}\nsafety:\n  confirmHighRisk: true\n  preventSelfCut: ${profile.safety.preventSelfCut}\n  scheduledDelaySeconds: ${profile.safety.scheduledDelaySeconds}\n  autoRollback: ${profile.safety.autoRollback}\nadvanced:\n  windowsSshService: sshd\n  linuxSshService: auto\n  macosSshLabel: com.openssh.sshd\n  stateDir: ${scalar(profile.advanced.stateDir)}\nlabels:\n  experience: guided\n`;
}

export function redactReport(report: Report): Report {
  const redacted = redactValue(structuredClone(report)) as Report;
  if (redacted.snapshot) {
    redacted.snapshot.hostname = "<redacted-host>";
    redacted.snapshot.targetUser = "<redacted-user>";
    if (redacted.snapshot.tailscale.ip) redacted.snapshot.tailscale.ip = "<redacted-ip>";
  }
  return redacted;
}

function redactValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(redactValue);
  }
  if (!value || typeof value !== "object") {
    return typeof value === "string" ? redactString(value) : value;
  }
  for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
    const normalized = key.toLowerCase();
    if (/(token|cookie|privatekey|password|credential|secret)/.test(normalized)) {
      (value as Record<string, unknown>)[key] = "<redacted>";
    } else if (typeof item === "string") {
      (value as Record<string, unknown>)[key] = redactString(item);
    } else {
      (value as Record<string, unknown>)[key] = redactValue(item);
    }
  }
  return value;
}

function redactString(value: string): string {
  if (value.includes("PRIVATE KEY")) return "<redacted-private-key-material>";
  return value
    .replace(/\b(?:\d{1,3}\.){3}\d{1,3}\b/g, "<redacted-ip>")
    .replace(/\b(?:[0-9a-f]{0,4}:){2,}[0-9a-f:]{0,4}\b/gi, "<redacted-ip>")
    .replace(/C:\\Users\\[^\\\s]+/gi, "C:\\Users\\<redacted-user>")
    .replace(/\/home\/[^/\s]+/g, "/home/<redacted-user>")
    .replace(/((?:token|cookie|password|secret)\s*[=:]\s*)[^\s,;]+/gi, "$1<redacted>");
}
