import { expect, test, type Page } from "@playwright/test";
import type { FailureReason } from "../src/types";

interface BridgeFaults {
  failStage?: "check" | "plan" | "verify";
  rejectKeys?: boolean;
  reason?: FailureReason;
  reportFailure?: "plan" | "verify";
  nullActions?: boolean;
}

async function installBridge(page: Page, mode: "failed" | "completed" | "running", faults: BridgeFaults = {}) {
  await page.addInitScript(({ mode, faults }) => {
    localStorage.setItem("ssh-launchpad-language", "zh-CN");
    const key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB controller";
    const trace = { polls: 0, rollback: "", release: false, mode, requests: [] as string[], validations: 0, exports: [] as string[], cardImports: 0 };
    (window as any).auditBridge = trace;
    const snapshot = {
      platform: "windows", arch: "amd64", hostname: "TEST", targetUser: "test", targetUserIsAdmin: true,
      isAdministrator: false, sessionTransport: "local", sshClient: { installed: true }, sshServer: { installed: true },
      sshService: { name: "sshd", installed: true, running: false, startPolicy: "Manual" }, sshPort: 22,
      sshConfigValid: true, sshAuthenticationChecked: true, sshPasswordAuthentication: false,
      sshKbdInteractiveAuthentication: false, sshPubkeyAuthentication: true,
      authorizedKeysChecked: true, authorizedKeysMatch: true, authorizedKeysCount: 1,
      tailscale: { installed: true, online: true, ip: "100.64.1.1" },
      firewall: { checked: true, enabled: true, provider: "windows-firewall", ports: [22], scopes: ["100.64.0.0/10", "fd7a:115c:a1e0::/48"] },
      network: { githubDns: true, tailscaleDns: true, proxySet: false, lanIps: [] }
    };
    const report = (success: boolean) => ({
      schemaVersion: 1, version: "0.2.6", started: new Date().toISOString(), finished: new Date().toISOString(), profileName: "test",
      id: "apply-test", stage: "apply", success, exitCode: success ? 0 : faults.reason ? 5 : 7,
      reasonCode: success ? undefined : faults.reason,
      journalPath: "C:\\ProgramData\\SSH Launchpad\\apply-test.journal.json",
      error: success ? "" : "injected partial failure", results: [{ actionId: "enable-sshd", status: success ? "completed" : "failed" }]
    });
    (window as any).go = { main: { App: {
      DefaultProfile: async () => ({ name: "test", labels: {} }),
      DiscoverPublicKeys: async () => faults.rejectKeys ? [] : [{ label: "controller.pub", path: "controller.pub", publicKey: key, generated: false }],
      ValidatePublicKey: async () => {
        trace.validations++;
        if (faults.rejectKeys) throw new Error("injected key rejection");
      },
      ExportProfile: async () => { trace.exports.push("profile"); return ""; },
      ImportPersonalCard: async () => { trace.cardImports++; return {}; },
      Run: async (request: any) => {
        trace.requests.push(request.stage);
        if (faults.failStage === request.stage) throw new Error(`injected ${request.stage} failure`);
        if (faults.reportFailure === "plan" && request.stage === "plan") {
          return { ...report(false), stage: "plan", exitCode: 3, reasonCode: "probe_failed", error: "injected typed plan failure with checksum word" };
        }
        const pending = faults.reportFailure === "verify" && request.stage === "verify";
        return {
          schemaVersion: 1, version: "0.2.6", started: new Date().toISOString(), finished: new Date().toISOString(), profileName: "test",
          id: request.stage + "-test", stage: request.stage, success: !pending, exitCode: pending ? 3 : 0,
          reasonCode: pending ? "verification_failed" : undefined, error: pending ? "injected remaining verification evidence" : "",
          snapshot: { ...snapshot, sshService: { ...snapshot.sshService, running: request.stage === "verify" } },
          plan: { timestamp: new Date().toISOString(), profileName: "test", platform: "windows", readOnly: true, digest: "a".repeat(64), noChanges: request.stage === "verify" && !pending, highestRisk: "medium", selfCutDetected: false, blockers: pending ? ["injected remaining verification evidence"] : [], actions: request.stage === "verify" ? faults.nullActions ? null : [] : [
            { id: "enable-sshd", operation: "enable_sshd", layer: "ssh-service", risk: "medium", mutating: true, requiresElevation: true, reversible: true, summary: "Start SSH", reason: "", selfCutRisk: false }
          ] }
        };
      },
      BeginElevatedApply: async () => ({ id: "job-test", state: mode, report: mode === "running" ? undefined : report(mode === "completed") }),
      ElevatedApplyStatus: async () => {
        trace.polls++;
        if (mode !== "running") throw new Error("terminal jobs must not be polled");
        return trace.release ? { id: "job-test", state: "completed", report: report(true) } : { id: "job-test", state: "running", events: [] };
      },
      DismissElevatedJob: async () => {},
      Rollback: async (path: string) => { trace.rollback = path; return { id: "rollback-test", stage: "rollback", journalPath: path, success: false, exitCode: 7, error: "injected recovery failure" }; }
    } } };
  }, { mode, faults });
  await page.goto("/");
}

async function bridge(page: Page, mode: "failed" | "completed" | "running", faults: BridgeFaults = {}) {
  await installBridge(page, mode, faults);
  await page.locator("#hero-start").click();
  await page.locator("#check-continue").click();
  await expect(page.locator("#open-install")).toBeEnabled();
  await page.locator("#open-install").click();
  await page.locator("#confirm-ack").check();
  await page.locator("#confirm-install").click();
}

for (const [reason, message] of [
  ["confirmation_required", "请先核对并确认当前变更方案"],
  ["mutation_busy", "另一个安装或恢复任务正在进行"],
  ["plan_changed", "这台电脑的状态刚发生了变化"]
] as const) {
  test(`Apply reason ${reason} is not guessed from its shared exit code`, async ({ page }) => {
    await bridge(page, "failed", { reason });
    await expect(page.getByText(message, { exact: false }).first()).toBeVisible();
    await expect(page.locator("#open-install")).toHaveCount(0);
  });
}

test("a structured native Plan failure retains its reason and does not classify error prose", async ({ page }) => {
  await installBridge(page, "completed", { reportFailure: "plan" });
  await page.locator("#hero-start").click();
  await page.locator("#check-continue").click();
  await expect(page.getByText("injected typed plan failure with checksum word", { exact: false }).first()).toBeVisible();
  await expect(page.locator("#plan-retry")).toBeEnabled();
  await expect(page.getByText("下载或离线文件未通过校验", { exact: false })).toHaveCount(0);
});

test("a native Verify drift report keeps remaining evidence rather than successful Apply data", async ({ page }) => {
  await bridge(page, "completed", { reportFailure: "verify" });
  await expect(page.getByRole("heading", { name: "还有项目没完成" })).toBeVisible();
  await expect(page.getByText("injected remaining verification evidence", { exact: false }).first()).toBeVisible();
  await expect(page.locator("#review-remaining")).toBeEnabled();
  await expect(page.locator("#finish")).toHaveCount(0);
});

test("Go's null no-op action list renders a successful Verify without crashing", async ({ page }) => {
  await bridge(page, "completed", { nullActions: true });
  await expect(page.getByRole("heading", { name: "本机配置已通过检查" })).toBeVisible();
  await expect(page.locator("#copy-handoff")).toBeVisible();
});

test("desktop Check failure is not replaced with a successful preview report", async ({ page }) => {
  await installBridge(page, "completed", { failStage: "check" });
  await page.locator("#hero-start").click();
  await expect(page.getByText("injected check failure", { exact: false }).first()).toBeVisible();
  await expect(page.locator("#check-continue")).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).auditBridge.requests)).toEqual(["check"]);
});

test("desktop Plan failure remains retryable instead of enabling a mock installation", async ({ page }) => {
  await installBridge(page, "completed", { failStage: "plan" });
  await page.locator("#hero-start").click();
  await page.locator("#check-continue").click();
  await expect(page.getByText("injected plan failure", { exact: false }).first()).toBeVisible();
  await expect(page.locator("#plan-retry")).toBeEnabled();
  await expect(page.locator("#open-install")).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).auditBridge.requests)).toEqual(["check", "plan"]);
});

test("desktop Verify rejection does not reuse the successful Apply report", async ({ page }) => {
  await bridge(page, "completed", { failStage: "verify" });
  await expect(page.getByText("injected verify failure", { exact: false }).first()).toBeVisible();
  await expect(page.getByRole("heading", { name: "本机配置已通过检查" })).toHaveCount(0);
  await expect(page.locator("#finish")).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).auditBridge.requests)).toEqual(["check", "plan", "verify"]);
});

test("desktop key rejection cannot fall back to the preview shape validator", async ({ page }) => {
  await installBridge(page, "completed", { rejectKeys: true });
  await page.locator("#hero-start").click();
  await page.locator("#check-continue").click();
  await page.locator("#public-key").fill("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB controller");
  await expect.poll(() => page.evaluate(() => (window as any).auditBridge.validations)).toBe(1);
  await expect(page.locator("#open-install")).toBeDisabled();
  expect(await page.evaluate(() => (window as any).auditBridge.requests)).toEqual(["check", "plan"]);
});

test("cancelled desktop file dialogs do not download preview files or report an import error", async ({ page }) => {
  const downloads: string[] = [];
  page.on("download", download => downloads.push(download.suggestedFilename()));
  await installBridge(page, "completed");
  await page.locator("#card-import-link").click();
  await expect.poll(() => page.evaluate(() => (window as any).auditBridge.cardImports)).toBe(1);
  await expect(page.getByText("装机卡导入失败", { exact: true })).toHaveCount(0);
  await expect(page.locator("#hero-start")).toBeVisible();
  await page.locator("#advanced-link").click();
  await page.locator("#export-profile").click();
  await expect.poll(() => page.evaluate(() => (window as any).auditBridge.exports)).toEqual(["profile"]);
  await expect(page.locator("#toast")).not.toHaveClass(/show/);
  expect(downloads).toEqual([]);
});

test("terminal failed bridge result retains its journal through Check and permits recovery", async ({ page }) => {
  await bridge(page, "failed");
  await expect(page.getByText("injected partial failure", { exact: false }).first()).toBeVisible();
  await expect(page.locator('#rollback-last')).toBeEnabled();
  await expect(page.locator('#open-install')).toHaveCount(0);
  await expect(page.locator('#review-remaining')).toBeEnabled();
  await page.screenshot({ path: test.info().outputPath('failed-recovery.png'), fullPage: true });
  expect(await page.evaluate(() => (window as any).auditBridge.polls)).toBe(0);
  await page.locator("#brand-home").click();
  await page.locator("#advanced-link").click();
  await expect(page.locator("#rollback-last")).toBeEnabled();
  await page.locator("#rollback-last").click();
  await page.locator('#confirm-dialog button[value="ok"]').click();
  await expect(page.locator("#toast")).toContainText("injected recovery failure");
  expect(await page.evaluate(() => (window as any).auditBridge.rollback)).toContain("apply-test.journal.json");
});

test("terminal successful bridge result proceeds directly to Verify without a job lookup", async ({ page }) => {
  await bridge(page, "completed");
  await expect(page.getByRole("heading", { name: "本机配置已通过检查" })).toBeVisible();
  expect(await page.evaluate(() => (window as any).auditBridge.polls)).toBe(0);
});

test("status deadline keeps tracking the active helper and blocks navigation", async ({ page }) => {
  await bridge(page, "running");
  await expect.poll(() => page.evaluate(() => (window as any).auditBridge.polls)).toBeGreaterThan(0);
  await page.evaluate(() => { const now = Date.now.bind(Date); Date.now = () => now() + 31 * 60 * 1000; });
  await expect(page.locator("#toast")).toContainText("可能仍在后台修改系统");
  await page.locator("#brand-home").click();
  await expect(page.locator("#hero-start")).toHaveCount(0);
  await page.evaluate(() => { (window as any).auditBridge.release = true; });
  await expect(page.getByRole("heading", { name: "本机配置已通过检查" })).toBeVisible();
});
