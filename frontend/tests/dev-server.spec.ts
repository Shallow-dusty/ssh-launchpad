import { expect, test } from "@playwright/test";
import { createServer } from "vite";
// @ts-expect-error These tests run in Node; the browser package does not ship Node typings.
import fs from "node:fs/promises";
// @ts-expect-error Vite's local server test needs Node file URL conversion.
import { fileURLToPath } from "node:url";

test("development loads Go-derived defaults without exposing sibling build artifacts", async ({ page, request }) => {
  const frontend = fileURLToPath(new URL("../", import.meta.url));
  const scratchBase = fileURLToPath(new URL("../../build/test-tmp/", import.meta.url));
  await fs.mkdir(scratchBase, { recursive: true });
  const scratch = await fs.mkdtemp(`${scratchBase}vite-boundary-`);
  const deniedFile = `${scratch}/private.txt`;
  const marker = "private-test-fixture-not-a-credential";
  await fs.writeFile(deniedFile, marker);
  const server = await createServer({
    root: frontend,
    configFile: `${frontend}vite.config.ts`,
    server: { host: "127.0.0.1", port: 0, strictPort: false }
  });
  try {
    await server.listen();
    const address = server.httpServer?.address() as { port: number };
    const base = `http://127.0.0.1:${address.port}`;
    await page.goto(base);
    await expect(page.locator("#hero-start")).toBeVisible();
    const contracts = fileURLToPath(new URL("../../build/contracts/wire-types.ts", import.meta.url));
    const allowed = await request.get(`${base}/@fs/${contracts.replace(/\\/g, "/")}`);
    expect(allowed.status()).toBe(200);
    expect(await allowed.text()).toContain("wireDefaultProfile");
    const denied = await request.get(`${base}/@fs/${deniedFile.replace(/\\/g, "/")}`);
    expect(denied.status()).toBe(403);
    expect(await denied.text()).not.toContain(marker);
  } finally {
    await server.close();
    await fs.rm(scratch, { recursive: true, force: true });
  }
});
