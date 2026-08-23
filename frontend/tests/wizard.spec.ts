import { expect, test } from "@playwright/test";
// @ts-expect-error Playwright runs tests in Node; the browser-only UI package intentionally does not ship Node typings.
import { readFile } from "node:fs/promises";

const controllerPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB personal-controller";
const tailscaleAuthKey = "tskey-" + "auth-k1234567890abcdef-1234567890abcdef";

async function startSetup(page: import("@playwright/test").Page): Promise<void> {
  await page.getByRole("button", { name: "开始", exact: true }).click();
  await expect(page.getByRole("heading", { name: "检查电脑" })).toBeVisible();
  await expect(page.getByRole("heading", { name: /这台电脑还没有开启远程连接|这台电脑已经可以被远程连接/ })).toBeVisible();
}

async function continueFromCheck(page: import("@playwright/test").Page): Promise<void> {
  await page.getByRole("button", { name: /继续|去验证/ }).click();
}

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.evaluate(() => localStorage.setItem("ssh-launchpad-language", "zh-CN"));
  await page.reload();
});

test("Chinese first-run wizard completes the recommended mock path", async ({ page }) => {
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await expect(page.getByRole("heading", { name: "让这台电脑可以被远程连接" })).toBeVisible();
  await startSetup(page);
  await expect(page.getByRole("button", { name: "继续" })).toBeVisible();
  await continueFromCheck(page);
  await expect(page.getByRole("heading", { name: "准备安装" })).toBeVisible();
  await expect(page.getByText("将会发生这些改动")).toBeVisible();
  await expect(page.getByRole("button", { name: "开始安装" })).toBeEnabled();

  await page.getByRole("button", { name: "开始安装" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  const installDialog = page.getByRole("dialog");
  await expect(installDialog.getByRole("button", { name: "开始安装", exact: true })).toBeDisabled();
  await page.getByRole("checkbox").check();
  await installDialog.getByRole("button", { name: "开始安装", exact: true }).click();

  await expect(page.getByRole("heading", { name: "这台电脑可以被远程连接了" })).toBeVisible();
  await expect(page.getByText(/ssh -p 22/)).toBeVisible();
  await expect(page.getByText("第一次连接时")).toBeVisible();
});

test("fresh computer without public keys asks neutrally and blocks installation until a key is supplied", async ({ page }) => {
  await page.goto("/?mock=no-public-key");
  await startSetup(page);
  await continueFromCheck(page);
  await expect(page.getByRole("heading", { name: "准备安装" })).toBeVisible();
  await expect(page.getByLabel("粘贴公钥")).toHaveValue("");
  await expect(page.getByText("还差一样：控制电脑的公钥")).toBeVisible();
  await expect(page.getByRole("button", { name: "开始安装" })).toBeDisabled();

  await page.getByLabel("粘贴公钥").fill(controllerPublicKey);
  await expect(page.getByRole("button", { name: "开始安装" })).toBeEnabled();
});

test("clearing a selected key clears profile state and blocks continuation", async ({ page }) => {
  await startSetup(page);
  await continueFromCheck(page);
  await page.locator("#change-key").click();
  await expect(page.getByLabel("粘贴公钥")).not.toHaveValue("");
  await page.getByLabel("粘贴公钥").fill("");
  await expect(page.getByText("还差一样：控制电脑的公钥")).toBeVisible();
  await expect(page.getByRole("button", { name: "开始安装" })).toBeDisabled();
  await expect(page.getByRole("heading", { name: "准备安装" })).toBeVisible();
});

test("guided mode can explicitly choose LAN instead of Tailscale", async ({ page }) => {
  await startSetup(page);
  await continueFromCheck(page);
  await page.locator("#change-network").click();
  await expect(page.getByRole("radio", { name: /Tailscale/ })).toBeChecked();
  await page.getByRole("radio", { name: /仅同一局域网/ }).check();
  await expect(page.getByText("仅同一局域网的设备", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "开始安装" })).toBeEnabled();
});

test("language switch persists and avoids mixed default navigation", async ({ page }) => {
  await page.getByRole("button", { name: "EN" }).click();
  await expect(page.getByRole("button", { name: "Get started", exact: true })).toBeVisible();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(page.getByRole("button", { name: "Get started", exact: true })).toBeVisible();
  await expect(page.getByText("让这台电脑可以被远程连接")).toHaveCount(0);
});

test("advanced mode imports a profile and preserves safe defaults", async ({ page }) => {
  await page.getByRole("button", { name: "高级选项" }).click();
  const profile = JSON.stringify({
    schemaVersion: 1,
    name: "import-test",
    target: { platform: "windows" },
    ssh: { enabled: true, port: 2222, publicKeys: [controllerPublicKey], passwordAuthentication: false },
    transport: { mode: "tailnet", install: false },
    exposure: { mode: "tailnet", customCidrs: [] },
    download: { strategy: "official", retries: 3 },
    safety: { confirmHighRisk: true, preventSelfCut: true, scheduledDelaySeconds: 20, autoRollback: true },
    advanced: {}
  });
  await page.locator("#profile-file").evaluate((element, content) => {
    const transfer = new DataTransfer();
    transfer.items.add(new File([content as string], "import.json", { type: "application/json" }));
    (element as HTMLInputElement).files = transfer.files;
    element.dispatchEvent(new Event("change", { bubbles: true }));
  }, profile);
  await expect(page.getByLabel("远程连接端口")).toHaveValue("2222");
  await expect(page.locator("#prevent-self-cut")).toBeChecked();
  await expect(page.locator("#auto-rollback")).toBeChecked();
});

test("personal card exports public settings and an optional Tailscale auth key", async ({ page }) => {
  await page.getByRole("button", { name: "高级选项" }).click();
  await page.getByLabel("名称").fill("实验电脑");
  await page.getByLabel("控制电脑", { exact: true }).fill("主控制电脑");
  await page.getByLabel("备注").fill("用于多设备实验");
  await page.getByLabel("Tailscale 授权码（可选）").fill(tailscaleAuthKey);
  await page.getByLabel("远程连接端口").fill("2222");
  await page.getByLabel("控制电脑公钥（每行一个）").fill(controllerPublicKey);

  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "导出装机卡" }).click();
  const saved = await download;
  expect(saved.suggestedFilename()).toBe("实验电脑.sshlaunchpad-card");
  const card = JSON.parse(await readFile(await saved.path(), "utf8"));
  expect(card).toMatchObject({
    schemaVersion: 1,
    kind: "ssh-launchpad-personal-card",
    displayName: "实验电脑",
    controllerName: "主控制电脑",
    note: "用于多设备实验",
    ssh: { port: 2222, publicKeys: [controllerPublicKey] },
    tailscale: { mode: "tailnet", authKey: tailscaleAuthKey }
  });
  expect(JSON.stringify(card)).not.toContain("PRIVATE KEY");
});

test("browser profile export preserves settings but omits a Tailscale auth key", async ({ page }) => {
  await page.getByRole("button", { name: "高级选项" }).click();
  await page.getByLabel("Tailscale 授权码（可选）").fill(tailscaleAuthKey);
  await page.getByLabel("远程连接端口").fill("2222");

  await page.evaluate(() => {
    const captured = window as typeof window & { __profileExport?: string };
    captured.__profileExport = "";
    URL.createObjectURL = (blob: Blob) => {
      void blob.text().then((text) => { captured.__profileExport = text; });
      return "blob:ssh-launchpad-profile-test";
    };
    URL.revokeObjectURL = () => undefined;
    HTMLAnchorElement.prototype.click = () => undefined;
  });
  await page.getByRole("button", { name: "导出 YAML 配置" }).click();
  await expect.poll(() => page.evaluate(() => (window as typeof window & { __profileExport?: string }).__profileExport)).not.toBe("");
  const yaml = await page.evaluate(() => (window as typeof window & { __profileExport?: string }).__profileExport ?? "");
  expect(yaml).toContain("port: 2222");
  expect(yaml).not.toContain(tailscaleAuthKey);
  expect(yaml).not.toMatch(/^\s*authKey:/m);
});

test("personal card import loads settings and starts the read-only guided check", async ({ page }) => {
  const card = JSON.stringify({
    schemaVersion: 1,
    kind: "ssh-launchpad-personal-card",
    displayName: "实验室卡片",
    controllerName: "控制端 A",
    note: "直接导入后继续",
    ssh: { port: 2222, publicKeys: [controllerPublicKey] },
    tailscale: { mode: "tailnet", install: true, authKey: tailscaleAuthKey }
  });
  await page.locator("#card-file").evaluate((element, content) => {
    const transfer = new DataTransfer();
    transfer.items.add(new File([content as string], "lab.sshlaunchpad-card", { type: "application/json" }));
    (element as HTMLInputElement).files = transfer.files;
    element.dispatchEvent(new Event("change", { bubbles: true }));
  }, card);

  await expect(page.getByRole("heading", { name: "检查电脑" })).toBeVisible();
  await expect(page.getByRole("heading", { name: /还没有开启远程连接/ })).toBeVisible();
  await page.getByRole("button", { name: "继续" }).click();
  await expect(page.getByRole("heading", { name: "准备安装" })).toBeVisible();
  await expect(page.locator(".card-loaded strong")).toContainText("实验室卡片");
  await page.locator("#change-key").click();
  await expect(page.getByLabel("粘贴公钥")).toHaveValue(controllerPublicKey);
  await expect(page.locator("body")).not.toContainText(tailscaleAuthKey);
});

test("personal card display name is rendered as text rather than HTML", async ({ page }) => {
  const displayName = `<img src=x onerror="window.__cardNameExecuted=true">`;
  await page.evaluate(() => {
    (window as typeof window & { __cardNameExecuted?: boolean }).__cardNameExecuted = false;
  });
  const card = JSON.stringify({
    schemaVersion: 1,
    kind: "ssh-launchpad-personal-card",
    displayName,
    ssh: { port: 22, publicKeys: [controllerPublicKey] },
    tailscale: { mode: "tailnet", install: false }
  });
  await page.locator("#card-file").evaluate((element, content) => {
    const transfer = new DataTransfer();
    transfer.items.add(new File([content as string], "text-only.sshlaunchpad-card", { type: "application/json" }));
    (element as HTMLInputElement).files = transfer.files;
    element.dispatchEvent(new Event("change", { bubbles: true }));
  }, card);

  await expect(page.getByRole("heading", { name: "检查电脑" })).toBeVisible();
  await page.getByRole("button", { name: "继续" }).click();
  await expect(page.getByRole("heading", { name: "准备安装" })).toBeVisible();
  await expect(page.locator(".card-loaded strong")).toContainText(displayName);
  await expect(page.locator(".card-loaded img")).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => Boolean((window as typeof window & { __cardNameExecuted?: boolean }).__cardNameExecuted))).toBe(false);
});

test("personal card import rejects private-key-shaped content", async ({ page }) => {
  const invalid = JSON.stringify({
    schemaVersion: 1,
    kind: "ssh-launchpad-personal-card",
    displayName: "bad",
    ssh: { port: 22, publicKeys: ["-----BEGIN OPENSSH PRIVATE KEY-----"] },
    tailscale: { mode: "tailnet", install: false }
  });
  await page.locator("#card-file").evaluate((element, content) => {
    const transfer = new DataTransfer();
    transfer.items.add(new File([content as string], "bad.sshlaunchpad-card", { type: "application/json" }));
    (element as HTMLInputElement).files = transfer.files;
    element.dispatchEvent(new Event("change", { bubbles: true }));
  }, invalid);
  await expect(page.locator("#toast")).toContainText("私钥");
  await expect(page.getByRole("heading", { name: "让这台电脑可以被远程连接" })).toBeVisible();
});

test("a blocked offline Tailscale plan still lets the user switch to LAN", async ({ page }) => {
  await page.goto("/?mock=tailnet-offline");
  await startSetup(page);
  await continueFromCheck(page);
  await expect(page.getByText("现在不能安全继续")).toBeVisible();
  await page.locator("#change-network").click();
  await page.getByRole("radio", { name: /仅同一局域网/ }).check();
  await expect(page.getByRole("button", { name: "开始安装" })).toBeEnabled();
});

test("repair mode lists unsafe firewall state instead of claiming the machine is healthy", async ({ page }) => {
  await page.evaluate(() => localStorage.setItem("ssh-launchpad-demo-ready", "true"));
  await page.goto("/?mock=unsafe-firewall");
  await page.getByRole("button", { name: "连接不上？检查并修复" }).click();
  await expect(page.getByRole("heading", { name: "检查问题" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "发现几处需要修复的问题" })).toBeVisible();
  await expect(page.getByText("防火墙只允许 Tailscale 网络的设备连接该端口")).toBeVisible();
});

test("cancelled UAC is a plain no-change result and can retry", async ({ page }) => {
  await page.goto("/?mock=uac-cancel");
  await startSetup(page);
  await continueFromCheck(page);
  await page.getByRole("button", { name: "开始安装" }).click();
  await page.getByRole("checkbox").check();
  await page.getByRole("dialog").getByRole("button", { name: "开始安装", exact: true }).click();
  await expect(page.getByRole("heading", { name: "已取消，电脑没有改动。" })).toBeVisible();
  await expect(page.getByRole("button", { name: "重新安装" })).toBeVisible();
});

test("second visit is idempotent and narrow layout remains usable", async ({ page }) => {
  await page.setViewportSize({ width: 480, height: 800 });
  await page.evaluate(() => localStorage.setItem("ssh-launchpad-demo-ready", "true"));
  await startSetup(page);
  await expect(page.getByRole("heading", { name: "这台电脑已经可以被远程连接" })).toBeVisible();
  await page.getByRole("button", { name: "去验证" }).click();
  await expect(page.getByRole("heading", { name: "这台电脑可以被远程连接了" })).toBeVisible();
  await expect(page.locator("body")).not.toHaveCSS("min-width", "900px");
});

test("keyboard skip link and live region remain available", async ({ page }) => {
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "跳到主要内容" })).toBeFocused();
  await page.getByRole("link", { name: "跳到主要内容" }).press("Enter");
  await expect(page.locator("#workspace")).toBeFocused();
  await expect(page.locator("#announcer")).toHaveAttribute("aria-live", "polite");
});
