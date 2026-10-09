import assert from "node:assert/strict";
import fs from "node:fs";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { commandsFor, executeCommands, profiles, scannerVersions, selectedCommands } from "../scripts/check.mjs";

function quiet(t) {
  const log = console.log;
  console.log = () => {};
  t.after(() => { console.log = log; });
}

test("quick/full/release retain local gates without publishing or installing", () => {
  assert.deepEqual(profiles.quick, ["go", "types", "docs"]);
  const release = selectedCommands("release", "linux").map(step => step.argv.join(" ")).join("\n");
  for (const gate of ["go test -p 1 ./...", "go test -p 1 -race ./...", "go vet ./...", "test:e2e", "check-docs.mjs", "shellcheck", "staticcheck@v0.7.0", "govulncheck@v1.6.0", "gosec@v2.28.0"]) assert.ok(release.includes(gate), gate);
  assert.doesNotMatch(release, /\baz\b|\bsudo\b|ssh-launchpad apply|release create|installer-upgrade-smoke/);
  assert.deepEqual(commandsFor("vuln"), [["go", "run", scannerVersions.govulncheck, "./..."]]);
  assert.deepEqual(commandsFor("security").at(-1), ["go", "run", scannerVersions.gosec, "-exclude-generated", "-exclude-dir=build", "-severity", "high", "./..."]);
});

test("native platform selection preserves Windows Pester and Unix syntax checks", () => {
  assert.deepEqual(commandsFor("scripts", "win32"), [["pwsh", "-NoProfile", "-NonInteractive", "-File", "scripts/check-pester.ps1"]]);
  assert.equal(commandsFor("scripts", "linux")[1].at(-2), "TestGeneratedUnixCommandSyntaxAndShellCheck");
  assert.ok(commandsFor("go", "win32").at(-1).includes("build/bin/ssh-launchpad.exe"));
});

test("a failed command stops later checks and is not converted into a pass", t => {
  quiet(t);
  const seen = [];
  const commands = [{ group: "test", argv: ["go", "first"] }, { group: "test", argv: ["go", "second"] }];
  assert.throws(() => executeCommands(commands, "/fixture", (command, args) => { seen.push(args[0]); return { status: 7 }; }), /failed \(7\)/);
  assert.deepEqual(seen, ["first"]);
});

test("launch errors propagate and only the fixed Windows pnpm shim uses a shell", t => {
  quiet(t);
  const options = [];
  executeCommands([{ group: "ui", argv: ["pnpm", "--dir", "frontend", "run", "typecheck"] }, { group: "go", argv: ["go", "vet", "./..."] }], "C:\\fixture", (command, args, option) => { options.push(option); return { status: 0 }; }, "win32");
  assert.equal(options[0].shell, true);
  assert.equal(options[1].shell, false);
  assert.throws(() => executeCommands([{ group: "go", argv: ["go"] }], "/fixture", () => ({ error: new Error("missing executable") })), /missing executable/);
});

test("unknown selections fail before executing commands", () => {
  assert.throws(() => selectedCommands("not-a-group"), /Unknown check group/);
  const result = spawnSync(process.execPath, ["scripts/check.mjs", "not-a-group"], { encoding: "utf8" });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /Unknown check group/);
});

test("CI and release use the same gates and keep scanner pins in one implementation", () => {
  const ci = fs.readFileSync(".github/workflows/ci.yml", "utf8");
  const release = fs.readFileSync(".github/workflows/release.yml", "utf8");
  for (const workflow of [ci, release]) {
    for (const group of ["go", "ui", "docs", "scripts"]) assert.ok(workflow.includes(`node scripts/check.mjs ${group}`), `${group} not shared`);
    assert.doesNotMatch(workflow, /govulncheck@latest|Invoke-Pester -Path tests -CI/);
  }
  assert.ok(ci.includes("node scripts/check.mjs vuln"));
  assert.ok(release.includes("node scripts/check.mjs security"));
  assert.ok(release.includes("node scripts/check.mjs race"));
});
