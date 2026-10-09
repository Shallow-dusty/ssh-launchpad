import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { checkLinks, headingAnchors, frozenSnapshots } from "../scripts/check-docs.mjs";

function fixture(t, files) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "ssh-launchpad-docs-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  for (const [name, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(root, name)), { recursive: true });
    fs.writeFileSync(path.join(root, name), content);
  }
  return root;
}

test("links resolve relative paths, root paths, local headings and encoded image names", t => {
  const root = fixture(t, {
    "README.md": "# Home\n[guide](docs/guide.md#说明)\n![image](<docs/screen shot.png>)\n[root](/docs/guide.md)\n[self](#home)\n",
    "docs/guide.md": "# 说明\n[home](../README.md#home)\n",
    "docs/screen shot.png": "fixture"
  });
  const result = checkLinks(root, ["README.md", "docs/guide.md"]);
  assert.deepEqual(result.errors, []);
  assert.equal(result.checked, 5);
});

test("duplicate, setext and explicit anchors match local references", () => {
  assert.deepEqual([...headingAnchors("# Same\n# Same\nHeading\n=======\n<a id=\"custom\"></a>\n## 中文 `Code`\n")], ["same", "same-1", "heading", "custom", "中文-code"]);
});

test("missing targets, missing headings and malformed URLs fail with source locations", t => {
  const root = fixture(t, { "README.md": "# Home\n[missing](no.md)\n[heading](#absent)\n[bad](bad%ZZ.md)\n" });
  const result = checkLinks(root, ["README.md"]);
  assert.equal(result.errors.length, 3);
  assert.match(result.errors[0], /README.md:2: missing target/);
  assert.match(result.errors[1], /README.md:3: missing heading/);
  assert.match(result.errors[2], /README.md:4: invalid link/);
});

test("reference links are validated, while code examples and external URLs are not fetched", t => {
  const root = fixture(t, {
    "README.md": "# Home\n[guide][Target]\n[Target]: docs/guide.md#target\n[unknown][absent]\n`[example](missing.md)`\n```md\n[fenced](missing.md)\n```\n[web](https://example.test/missing)\n",
    "docs/guide.md": "# Target\n"
  });
  const result = checkLinks(root, ["README.md"]);
  assert.equal(result.errors.length, 1);
  assert.match(result.errors[0], /undefined reference absent/);
  assert.equal(result.checked, 2);
});

test("only the two documented verbatim snapshots are excluded", t => {
  const files = Object.fromEntries([...frozenSnapshots].map(name => [name, "[historical path](missing.md)"]));
  files["docs/90.Archive/01.history/ARCHIVE_NOTE.md"] = "[broken](missing.md)";
  const root = fixture(t, files);
  const result = checkLinks(root, Object.keys(files));
  assert.equal(result.errors.length, 1);
  assert.match(result.errors[0], /ARCHIVE_NOTE.md/);
});
