import { expect, test, type Page } from "@playwright/test";

async function bridge(page: Page, mode: "failed" | "completed" | "running") {
  await page.addInitScript(({ mode }) => {
    localStorage.setItem("ssh-launchpad-language", "zh-CN");
    const key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB controller";
    const trace = { polls: 0, rollback: "", release: false, mode };
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
      id: "apply-test", stage: "apply", success, exitCode: success ? 0 : 7,
      journalPath: "C:\\ProgramData\\SSH Launchpad\\apply-test.journal.json",
      error: success ? "" : "injected partial failure", results: [{ actionId: "enable-sshd", status: success ? "completed" : "failed" }]
    });
    (window as any).go = { main: { App: {
      DefaultProfile: async () => ({ name: "test", labels: {} }),
      DiscoverPublicKeys: async () => [{ label: "controller.pub", path: "controller.pub", publicKey: key, generated: false }],
      ValidatePublicKey: async () => null,
      Run: async (request: any) => ({
        id: request.stage + "-test", stage: request.stage, success: true, exitCode: 0,
        snapshot: { ...snapshot, sshService: { ...snapshot.sshService, running: request.stage === "verify" } },
        plan: { digest: "a".repeat(64), noChanges: request.stage === "verify", highestRisk: "medium", selfCutDetected: false, blockers: [], actions: request.stage === "verify" ? [] : [
          { id: "enable-sshd", operation: "enable_sshd", layer: "ssh-service", risk: "medium", mutating: true, requiresElevation: true, reversible: true, summary: "Start SSH", reason: "", selfCutRisk: false }
        ] }
      }),
      BeginElevatedApply: async () => ({ id: "job-test", state: mode, report: mode === "running" ? undefined : report(mode === "completed") }),
      ElevatedApplyStatus: async () => {
        trace.polls++;
        if (mode !== "running") throw new Error("terminal jobs must not be polled");
        return trace.release ? { id: "job-test", state: "completed", report: report(true) } : { id: "job-test", state: "running", events: [] };
      },
      DismissElevatedJob: async () => {},
      Rollback: async (path: string) => { trace.rollback = path; return { id: "rollback-test", stage: "rollback", journalPath: path, success: false, exitCode: 7, error: "injected recovery failure" }; }
    } } };
  }, { mode });
  await page.goto("/");
  await page.locator("#hero-start").click();
  await page.locator("#check-continue").click();
  await expect(page.locator("#open-install")).toBeEnabled();
  await page.locator("#open-install").click();
  await page.locator("#confirm-ack").check();
  await page.locator("#confirm-install").click();
}

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
