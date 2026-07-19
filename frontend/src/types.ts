export type Stage = "check" | "plan" | "apply" | "verify";
export type Risk = "low" | "medium" | "high" | "critical";

export interface Profile {
  schemaVersion: number;
  name: string;
  target: { platform: string; wsl?: boolean };
  ssh: { enabled: boolean; port: number; publicKeys: string[]; passwordAuthentication: boolean };
  transport: { mode: string; install: boolean };
  exposure: { mode: string; customCidrs: string[] };
  download: {
    strategy: string;
    mirrorBaseUrl: string;
    proxyUrl: string;
    offlineBundle: string;
    cacheDir: string;
    retries: number;
  };
  safety: {
    confirmHighRisk: boolean;
    preventSelfCut: boolean;
    scheduledDelaySeconds: number;
    autoRollback: boolean;
  };
  advanced: {
    windowsSshService: string;
    linuxSshService: string;
    macosSshLabel: string;
    stateDir: string;
  };
  labels: Record<string, string>;
}

export interface Snapshot {
  platform: string;
  arch: string;
  hostname: string;
  isAdministrator: boolean;
  sessionTransport: string;
  packageManager?: string;
  sshClient: { installed: boolean; version?: string };
  sshServer: { installed: boolean; version?: string };
  sshService: { name?: string; installed: boolean; running: boolean; startPolicy?: string };
  sshPort?: number;
  sshConfigValid: boolean;
  firewall: { provider?: string; ports?: number[]; scopes?: string[] };
  tailscale: { installed: boolean; online: boolean; ip?: string; state?: string };
  network: { githubDns: boolean; tailscaleDns: boolean; proxySet: boolean };
  warnings?: string[];
}

export interface PlanAction {
  id: string;
  operation: string;
  layer: string;
  risk: Risk;
  summary: string;
  reason: string;
  mutating: boolean;
  requiresElevation: boolean;
  selfCutRisk: boolean;
  reversible: boolean;
}

export interface Plan {
  noChanges: boolean;
  highestRisk: Risk;
  selfCutDetected: boolean;
  actions: PlanAction[];
  warnings?: string[];
}

export interface Report {
  id: string;
  stage: Stage;
  success: boolean;
  exitCode: number;
  profileName: string;
  snapshot?: Snapshot;
  plan?: Plan;
  journalPath?: string;
  results?: Array<{ actionId: string; status: string; output?: string; error?: string }>;
  warnings?: string[];
  error?: string;
}

export interface DesktopRequest {
  stage: Stage;
  profile: Profile;
  confirmed: boolean;
  allowSelfCut: boolean;
  scheduleRisky: boolean;
  externalVerify: string;
}

export interface PublicKeyInfo {
  label: string;
  path: string;
  publicKey: string;
  privateKeyPath?: string;
  generated: boolean;
}

export interface ElevatedJob {
  id: string;
  state: "waiting-for-permission" | "running" | "completed" | "failed" | "cancelled";
  report?: Report;
  error?: string;
  events?: Array<{ kind: string; message: string; actionId?: string }>;
}

export interface UpdateInfo {
  currentVersion: string;
  latestVersion: string;
  available: boolean;
  url: string;
  channel: string;
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          DefaultProfile(): Promise<Profile>;
          Run(request: DesktopRequest): Promise<Report>;
          BeginElevatedApply(request: DesktopRequest): Promise<ElevatedJob>;
          ElevatedApplyStatus(id: string): Promise<ElevatedJob>;
        DismissElevatedJob(id: string): Promise<void>;
        CheckForUpdate(): Promise<UpdateInfo>;
          Rollback(journalPath: string): Promise<Report>;
          ExportReport(report: Report): Promise<string>;
          ImportProfile(): Promise<Profile>;
          ExportProfile(profile: Profile): Promise<string>;
          DiscoverPublicKeys(): Promise<PublicKeyInfo[]>;
          GenerateControllerKey(label: string): Promise<PublicKeyInfo>;
          ImportPublicKey(): Promise<PublicKeyInfo>;
          ExportPairingFile(publicKey: string): Promise<string>;
        };
      };
    };
    runtime?: {
      EventsOn(name: string, callback: (event: { kind: string; message: string; actionId?: string }) => void): void;
    };
  }
}
