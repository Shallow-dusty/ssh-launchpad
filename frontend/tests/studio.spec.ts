import { expect, test } from "@playwright/test";

test("common path exposes status, plan, apply confirmation, and advanced safety", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Host status" })).toBeVisible();
  await page.getByRole("button", { name: "Run check" }).click();
  await expect(page.getByText("DEMO-HOST")).toBeVisible();

  await page.getByRole("button", { name: "Plan", exact: true }).click();
  await page.getByRole("button", { name: "Refresh plan" }).click();
  await expect(page.getByRole("heading", { name: "What Apply would change" })).toBeVisible();
  await page.getByRole("button", { name: "Review & apply" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByLabel("Independent verification target")).toBeVisible();
  await expect(page.getByRole("button", { name: "Apply changes" })).toBeDisabled();
  await page.getByLabel("I reviewed the actions, rollback coverage, and control-channel risk.").check();
  await expect(page.getByRole("button", { name: "Apply changes" })).toBeEnabled();
  await page.getByRole("button", { name: "Cancel" }).click();

  await page.getByRole("button", { name: "Advanced" }).click();
  await expect(page.getByLabel("Target layer")).toBeVisible();
  await expect(page.getByLabel("SSH port")).toHaveValue("22");
  await page.getByText("Safety and authentication").click();
  await expect(page.getByLabel("Block active-channel self-cut")).toBeChecked();
});

test("keyboard navigation and semantic status remain usable", async ({ page }) => {
  await page.goto("/");
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to workspace" })).toBeFocused();
  await page.getByRole("link", { name: "Skip to workspace" }).press("Enter");
  await expect(page.locator("#workspace")).toBeFocused();
  await expect(page.locator("#announcer")).toHaveAttribute("aria-live", "polite");
});
