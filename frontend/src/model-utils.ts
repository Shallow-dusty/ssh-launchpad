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

// Keep the browser preview's export contract aligned with the desktop export:
// preserve user settings, but never serialize the optional auth key.
export function profileToYAML(profile: Profile): string {
  const scalar = (value: unknown) => JSON.stringify(value);
  const list = (name: string, values: string[]) => values.length
    ? `${name}:\n${values.map((value) => `    - ${scalar(value)}`).join("\n")}`
    : `${name}: []`;
  const labels = Object.entries(profile.labels);
  const labelBlock = labels.length
    ? labels.map(([key, value]) => `  ${scalar(key)}: ${scalar(value)}`).join("\n")
    : "  {}";
  const wsl = profile.target.wsl ? "\n  wsl: true" : "";
  return `schemaVersion: ${profile.schemaVersion}
name: ${scalar(profile.name)}
target:
  platform: ${scalar(profile.target.platform)}${wsl}
ssh:
  enabled: ${profile.ssh.enabled}
  port: ${profile.ssh.port}
  ${list("publicKeys", profile.ssh.publicKeys)}
  passwordAuthentication: ${profile.ssh.passwordAuthentication}
transport:
  mode: ${scalar(profile.transport.mode)}
  install: ${profile.transport.install}
exposure:
  mode: ${scalar(profile.exposure.mode)}
  ${list("customCidrs", profile.exposure.customCidrs)}
download:
  strategy: ${scalar(profile.download.strategy)}
  mirrorBaseUrl: ${scalar(profile.download.mirrorBaseUrl)}
  proxyUrl: ${scalar(profile.download.proxyUrl)}
  offlineBundle: ${scalar(profile.download.offlineBundle)}
  offlineSha256: ${scalar(profile.download.offlineSha256)}
  cacheDir: ${scalar(profile.download.cacheDir)}
  retries: ${profile.download.retries}
safety:
  confirmHighRisk: ${profile.safety.confirmHighRisk}
  preventSelfCut: ${profile.safety.preventSelfCut}
  scheduledDelaySeconds: ${profile.safety.scheduledDelaySeconds}
  autoRollback: ${profile.safety.autoRollback}
advanced:
  windowsSshService: ${scalar(profile.advanced.windowsSshService)}
  linuxSshService: ${scalar(profile.advanced.linuxSshService)}
  macosSshLabel: ${scalar(profile.advanced.macosSshLabel)}
  stateDir: ${scalar(profile.advanced.stateDir)}
labels:
${labelBlock}
`;
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
    if (/(token|cookie|privatekey|password|credential|secret|authkey)/.test(normalized) && typeof item === "string") {
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
    .replace(/\btskey-auth-[A-Za-z0-9_+/=-]+/gi, "<redacted-tailscale-auth-key>")
    .replace(/\b(?:\d{1,3}\.){3}\d{1,3}\b/g, "<redacted-ip>")
    .replace(/\b(?:[0-9a-f]{0,4}:){2,}[0-9a-f:]{0,4}\b/gi, "<redacted-ip>")
    .replace(/C:\\Users\\[^\\\s]+/gi, "C:\\Users\\<redacted-user>")
    .replace(/\/home\/[^/\s]+/g, "/home/<redacted-user>")
    .replace(/((?:token|cookie|password|secret|authkey)\s*[=:]\s*)[^\s,;]+/gi, "$1<redacted>");
}
