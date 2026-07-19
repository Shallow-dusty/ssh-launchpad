import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.evaluate(() => localStorage.setItem("ssh-launchpad-language", "zh-CN"));
  await page.reload();
});

test("Chinese first-run wizard completes the recommended mock path", async ({ page }) => {
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await expect(page.getByRole("heading", { name: "你想做什么？" })).toBeVisible();
  await expect(page.getByText("被连接电脑")).toBeVisible();
  await page.getByRole("button", { name: /让这台电脑可以被远程连接/ }).click();
  await expect(page.getByRole("heading", { name: "检查电脑" })).toBeVisible();
  await expect(page.getByText(/还差 \d+ 步/)).toBeVisible();
  await page.getByRole("button", { name: "继续" }).click();
  await expect(page.getByRole("heading", { name: "推荐方案" })).toBeVisible();
  await expect(page.getByText(/私钥留在控制电脑/)).toBeVisible();
  await page.getByRole("button", { name: "使用推荐设置" }).click();
  await expect(page.getByRole("heading", { name: "将要做这些事" })).toBeVisible();
  await expect(page.getByText("谁能连接")).toBeVisible();
  await page.getByRole("button", { name: "开始安全安装" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByRole("button", { name: /继续并弹出 Windows 权限确认/ })).toBeDisabled();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: /继续并弹出 Windows 权限确认/ }).click();
  await expect(page.getByRole("heading", { name: /已准备好|还差/ })).toBeVisible();
  await expect(page.getByText(/ssh -p 22/)).toBeVisible();
  await expect(page.getByText(/主机指纹/)).toBeVisible();
});

test("language switch persists and avoids mixed default navigation", async ({ page }) => {
  await page.getByLabel("语言").selectOption("en");
  await expect(page.getByRole("heading", { name: "What would you like to do?" })).toBeVisible();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(page.getByRole("button", { name: /Let me connect to this computer remotely/ })).toBeVisible();
  await expect(page.getByText("让这台电脑可以被远程连接")).toHaveCount(0);
});

test("advanced mode imports a profile and preserves safe defaults", async ({ page }) => {
  await page.getByRole("button", { name: /高级模式/ }).click();
  const profile = JSON.stringify({
      schemaVersion: 1,
      name: "import-test",
      target: { platform: "windows" },
      ssh: { enabled: true, port: 2222, publicKeys: ["ssh-ed25519 AAAA test"], passwordAuthentication: false },
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

test("cancelled UAC is a plain no-change result and can retry", async ({ page }) => {
  await page.goto("/?mock=uac-cancel");
  await page.getByRole("button", { name: /让这台电脑可以被远程连接/ }).click();
  await page.getByRole("button", { name: "继续" }).click();
  await page.getByRole("button", { name: "使用推荐设置" }).click();
  await page.getByRole("button", { name: "开始安全安装" }).click();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: /继续并弹出 Windows 权限确认/ }).click();
  await expect(page.getByRole("heading", { name: "没有改动" })).toBeVisible();
  await expect(page.getByText(/取消了 Windows 权限确认/)).toBeVisible();
  await expect(page.getByRole("button", { name: "重试" })).toBeVisible();
});

test("second visit is idempotent and narrow layout remains usable", async ({ page }) => {
  await page.setViewportSize({ width: 480, height: 800 });
  await page.evaluate(() => localStorage.setItem("ssh-launchpad-demo-ready", "true"));
  await page.getByRole("button", { name: /让这台电脑可以被远程连接/ }).click();
  await expect(page.getByRole("heading", { name: "已准备好" })).toBeVisible();
  await page.getByRole("button", { name: "继续" }).click();
  await page.getByRole("button", { name: "使用推荐设置" }).click();
  await expect(page.getByText(/已经配置好|无需重复改动/)).toBeVisible();
  await expect(page.locator("body")).not.toHaveCSS("min-width", "900px");
});

test("keyboard skip link and live region remain available", async ({ page }) => {
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "跳到主要内容" })).toBeFocused();
  await page.getByRole("link", { name: "跳到主要内容" }).press("Enter");
  await expect(page.locator("#workspace")).toBeFocused();
  await expect(page.locator("#announcer")).toHaveAttribute("aria-live", "polite");
});
