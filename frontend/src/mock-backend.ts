import { delay } from "./browser-utils";
import type { DesktopRequest, PlanAction, PublicKeyInfo, Report, Snapshot } from "./types";

export async function mockRun(request: DesktopRequest): Promise<Report> {
  await delay(180);
  const configured = localStorage.getItem("ssh-launchpad-demo-ready") === "true";
  const snapshot: Snapshot = {
    platform: "windows",
    arch: "amd64",
    hostname: "HOME-PC",
    targetUser: "demo",
    targetUserIsAdmin: true,
    isAdministrator: false,
    sessionTransport: "local",
    packageManager: "winget",
    sshClient: { installed: true, version: "OpenSSH_9.x" },
    sshServer: { installed: configured },
    sshService: { name: "sshd", installed: configured, running: configured, startPolicy: configured ? "Automatic" : "Manual" },
    sshPort: configured ? request.profile.ssh.port : 0,
    sshConfigValid: true,
    sshAuthenticationChecked: true,
    sshPasswordAuthentication: false,
    sshKbdInteractiveAuthentication: false,
    sshPubkeyAuthentication: true,
    sshAuthorizedKeysFileChecked: true,
    sshAuthorizedKeysFile: "C:\\ProgramData\\ssh\\administrators_authorized_keys",
    authorizedKeysChecked: true,
    authorizedKeysMatch: configured,
    authorizedKeysCount: configured ? 1 : 0,
    firewall: { checked: true, enabled: true, provider: "windows-firewall", ports: configured ? [request.profile.ssh.port] : [], scopes: configured ? ["100.64.0.0/10", "fd7a:115c:a1e0::/48"] : [] },
    tailscale: { installed: true, online: true, ip: "100.64.10.25", state: "Running" },
    network: { githubDns: true, tailscaleDns: true, proxySet: false, lanIps: ["192.168.1.25"], lanScopes: ["192.168.1.0/24"] }
  };
  const actions = configured ? [] : mockActions(request.profile.ssh.port);
  return {
    id: `${request.stage}-${Date.now()}`,
    stage: request.stage,
    success: request.stage !== "verify" || configured,
    exitCode: request.stage === "verify" && !configured ? 3 : 0,
    profileName: request.profile.name,
    snapshot,
    plan: { digest: `mock-${request.profile.ssh.port}-${actions.length}`, noChanges: actions.length === 0, highestRisk: actions.length ? "high" : "low", selfCutDetected: false, actions }
  };
}

export function mockActions(port: number): PlanAction[] {
  return [
    { id: "install-ssh", operation: "install_ssh", layer: "ssh-packages", risk: "medium", summary: "Install OpenSSH", reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: false },
    { id: "configure-sshd", operation: "configure_sshd", layer: "ssh-config", risk: "high", summary: "Configure SSH", reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true },
    { id: "configure-authorized-keys", operation: "configure_keys", layer: "authentication", risk: "high", summary: "Add controller key", reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true },
    { id: "enable-sshd", operation: "enable_sshd", layer: "ssh-service", risk: "medium", summary: "Start service", reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true },
    { id: "configure-firewall", operation: "configure_firewall", layer: "firewall", risk: "high", summary: `Allow ${port}`, reason: "", mutating: true, requiresElevation: true, selfCutRisk: false, reversible: true }
  ];
}

export function mockPublicKey(generated = false): PublicKeyInfo {
  const basename = "id_ed25519_ssh_launchpad";
  return {
    label: `${basename}.pub`,
    path: `C:\\Users\\demo\\.ssh\\${basename}.pub`,
    privateKeyPath: `C:\\Users\\demo\\.ssh\\${basename}`,
    generated,
    publicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB mock-controller"
  };
}
