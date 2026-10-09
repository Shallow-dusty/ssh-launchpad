import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";

export const scannerVersions = {
  staticcheck: "honnef.co/go/tools/cmd/staticcheck@v0.7.0",
  govulncheck: "golang.org/x/vuln/cmd/govulncheck@v1.6.0",
  gosec: "github.com/securego/gosec/v2/cmd/gosec@v2.28.0"
};

export const profiles = {
  quick: ["go", "types", "docs"],
  full: ["go", "race", "ui", "docs", "scripts"],
  release: ["go", "race", "ui", "docs", "scripts", "security"]
};

export function commandsFor(group, platform = process.platform) {
  switch (group) {
    case "go": return [
      ["go", "test", "-p", "1", "./..."],
      ["go", "vet", "./..."],
      ["go", "build", "-trimpath", "-o", `build/bin/ssh-launchpad${platform === "win32" ? ".exe" : ""}`, "./cmd/ssh-launchpad"]
    ];
    case "race": return [["go", "test", "-p", "1", "-race", "./..."]];
    case "types": return [["pnpm", "--dir", "frontend", "run", "typecheck"]];
    case "ui": return [
      ["pnpm", "--dir", "frontend", "audit", "--audit-level", "high"],
      ["pnpm", "--dir", "frontend", "run", "test:e2e"]
    ];
    case "docs": return [
      [process.execPath, "--test", "tests/docs-links.test.mjs", "tests/check-runner.test.mjs"],
      [process.execPath, "scripts/check-docs.mjs"]
    ];
    case "scripts": return platform === "win32"
      ? [["pwsh", "-NoProfile", "-NonInteractive", "-File", "scripts/check-pester.ps1"]]
      : [
        ["shellcheck", "scripts/bootstrap.sh", "scripts/new-offline-pack.sh", "packaging/launchers/Start SSH Launchpad.command"],
        ["go", "test", "-p", "1", "./internal/launchpad", "-run", "TestGeneratedUnixCommandSyntaxAndShellCheck", "-count=1"]
      ];
    case "vuln": return [["go", "run", scannerVersions.govulncheck, "./..."]];
    case "security": return [
      ["go", "run", scannerVersions.staticcheck, "./..."],
      ...commandsFor("vuln", platform),
      // build/ holds ignored, standalone repro tools and generated artifacts,
      // not production packages; gosec walks it even when go list ignores it.
      ["go", "run", scannerVersions.gosec, "-exclude-generated", "-exclude-dir=build", "-severity", "high", "./..."]
    ];
    default: throw new Error(`Unknown check group: ${group}`);
  }
}

export function selectedCommands(selection, platform = process.platform) {
  const groups = Object.hasOwn(profiles, selection) ? profiles[selection] : [selection];
  return groups.flatMap(group => commandsFor(group, platform).map(argv => ({ group, argv })));
}

export function executeCommands(commands, root, spawn = spawnSync, platform = process.platform) {
  for (const { group, argv } of commands) {
    const [command, ...args] = argv;
    console.log(`[check:${group}] ${argv.join(" ")}`);
    // pnpm is a .cmd shim on Windows. Only this fixed command uses a shell;
    // selections are validated before execution and no user arguments are forwarded.
    const result = spawn(command, args, { cwd: root, stdio: "inherit", shell: platform === "win32" && command === "pnpm" });
    if (result.error) throw result.error;
    if (result.status !== 0) throw new Error(`${command} failed (${result.status ?? result.signal ?? "unknown"})`);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try {
    if (process.argv.length > 3) throw new Error("Usage: node scripts/check.mjs [quick|full|release|go|race|types|ui|docs|scripts|vuln|security]");
    const selection = process.argv[2] ?? "quick";
    const commands = selectedCommands(selection);
    const root = fileURLToPath(new URL("../", import.meta.url));
    fs.mkdirSync(path.join(root, "build", "bin"), { recursive: true });
    executeCommands(commands, root);
    console.log(`PASS: ${selection} local checks. No live system acceptance or release publishing was performed.`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
