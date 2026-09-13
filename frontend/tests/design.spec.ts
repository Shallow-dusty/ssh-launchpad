import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('ssh-launchpad-language', 'zh-CN'));
});

test('home explains the flow and fits narrow, light and dark viewports', async ({ page }, info) => {
  await page.goto('/');
  await expect(page.getByRole('list', { name: '接下来会发生什么' })).toBeVisible();
  await page.screenshot({ path: info.outputPath('home-light.png'), fullPage: true });
  await page.locator('#theme-toggle').click();
  await page.screenshot({ path: info.outputPath('home-dark.png'), fullPage: true });
  await page.setViewportSize({ width: 360, height: 800 });
  await page.screenshot({ path: info.outputPath('home-narrow.png'), fullPage: true });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
});

test('review step indicates current location and focuses its heading', async ({ page }, info) => {
  await page.goto('/');
  await page.locator('#hero-start').click();
  await page.locator('#check-continue').click();
  await expect(page.locator('#open-install')).toBeEnabled();
  await expect(page.locator('[aria-current="step"]')).toContainText('准备安装');
  await expect(page.locator('.wizard-title')).toBeFocused();
  await page.screenshot({ path: info.outputPath('review.png'), fullPage: true });
});

test('unfinished verify offers another review rather than a false finish', async ({ page }, info) => {
  await page.addInitScript(() => localStorage.setItem('ssh-launchpad-demo-ready', 'true'));
  await page.goto('/?mock=verify-pending');
  await page.locator('#hero-start').click();
  await page.locator('#check-continue').click();
  await expect(page.getByRole('heading', { name: '还有项目没完成' })).toBeVisible();
  await expect(page.locator('#finish')).toHaveCount(0);
  await expect(page.locator('#review-remaining')).toBeEnabled();
  await page.screenshot({ path: info.outputPath('verification-pending.png'), fullPage: true });
  await page.locator('#review-remaining').click();
  await expect(page.locator('.wizard-title')).toContainText('准备安装');
});

test('successful handoff states the local verification boundary', async ({ page, context }, info) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.addInitScript(() => localStorage.setItem('ssh-launchpad-demo-ready', 'true'));
  await page.goto('/');
  await page.locator('#hero-start').click();
  await page.locator('#check-continue').click();
  await expect(page.locator('#copy-handoff')).toBeVisible();
  await page.locator('#copy-handoff').click();
  const text = await page.evaluate(() => navigator.clipboard.readText());
  expect(text).toContain('ssh -p 22');
  expect(text).toContain('仍需从控制电脑实际连接');
  expect(text).not.toContain('tskey-');
  await page.screenshot({ path: info.outputPath('handoff.png'), fullPage: true });
});
