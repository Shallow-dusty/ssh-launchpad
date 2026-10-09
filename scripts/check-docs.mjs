import fs from "node:fs";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";

// These are verbatim pre-organization snapshots, not active documentation.
// Their original path context is documented by the adjacent ARCHIVE_NOTE.
export const frozenSnapshots = new Set([
  "docs/90.Archive/02.pre-organization-baseline/STATUS-2026-09-16.md",
  "docs/90.Archive/02.pre-organization-baseline/ROADMAP.md"
]);

function prose(markdown) {
  let fence = "";
  return markdown.split("\n").map(line => {
    const marker = line.match(/^\s{0,3}(`{3,}|~{3,})/);
    if (marker && (!fence || marker[1][0] === fence[0] && marker[1].length >= fence.length)) {
      fence = fence ? "" : marker[1];
      return "";
    }
    return fence ? "" : line;
  }).join("\n");
}

export function headingAnchors(markdown) {
  const lines = prose(markdown).split("\n");
  const anchors = new Set();
  const duplicates = new Map();
  const add = text => {
    const slug = text
      .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
      .replace(/<[^>]*>/g, "")
      .replace(/&amp;/g, "&")
      .toLowerCase()
      .replace(/[^\p{L}\p{N}_\-\s]/gu, "")
      .replace(/\s/g, "-");
    const count = duplicates.get(slug) ?? 0;
    duplicates.set(slug, count + 1);
    anchors.add(count ? `${slug}-${count}` : slug);
  };
  for (let index = 0; index < lines.length; index++) {
    const heading = lines[index].match(/^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$/);
    if (heading) add(heading[1]);
    else if (/^\s{0,3}(?:=+|-+)\s*$/.test(lines[index]) && index > 0 && lines[index - 1].trim()) add(lines[index - 1].trim());
    for (const match of lines[index].matchAll(/<a\s+[^>]*(?:id|name)=["']([^"']+)["'][^>]*>/g)) anchors.add(match[1]);
  }
  return anchors;
}

function destinations(markdown) {
  const text = prose(markdown).replace(/(`+)(?!`)[\s\S]*?\1/g, match => match.replace(/[^\n]/g, " "));
  const references = new Map();
  const links = [];
  const key = value => value.trim().toLowerCase().replace(/\s+/g, " ");
  const destination = raw => raw.startsWith("<") ? raw.slice(1, raw.indexOf(">")) : raw.trim().split(/\s+["']/)[0];
  for (const match of text.matchAll(/^\s{0,3}\[([^\]]+)\]:\s*(<[^>]+>|\S+)/gm)) {
    const href = destination(match[2]);
    references.set(key(match[1]), href);
    links.push({ href, index: match.index });
  }
  for (const match of text.matchAll(/!?\[[^\]\n]*\]\(\s*(<[^>\n]+>|[^)\n]+)\)/g)) links.push({ href: destination(match[1]), index: match.index });
  for (const match of text.matchAll(/!?\[([^\]\n]+)\]\[([^\]\n]*)\]/g)) {
    const id = key(match[2] || match[1]);
    links.push({ href: references.get(id), reference: id, index: match.index });
  }
  return links.map(link => ({ ...link, line: text.slice(0, link.index).split("\n").length }));
}

export function checkLinks(root, files) {
  const errors = [];
  let checked = 0;
  const anchorCache = new Map();
  for (const file of files) {
    if (frozenSnapshots.has(file) || !fs.existsSync(path.join(root, file))) continue;
    for (const link of destinations(fs.readFileSync(path.join(root, file), "utf8"))) {
      if (!link.href) {
        errors.push(`${file}:${link.line}: undefined reference ${link.reference ?? ""}`);
        continue;
      }
      if (/^(?:[a-z][a-z\d+.-]*:|\/\/)/i.test(link.href)) continue;
      checked++;
      try {
        const [relative, fragment] = link.href.split("#", 2);
        const name = decodeURIComponent(relative.split("?", 1)[0]);
        const target = name.startsWith("/") ? path.join(root, name.slice(1)) : path.resolve(root, path.dirname(file), name || path.basename(file));
        if (!fs.existsSync(target)) {
          errors.push(`${file}:${link.line}: missing target ${link.href}`);
          continue;
        }
        if (fragment) {
          if (!target.endsWith(".md")) continue;
          if (!anchorCache.has(target)) anchorCache.set(target, headingAnchors(fs.readFileSync(target, "utf8")));
          if (!anchorCache.get(target).has(decodeURIComponent(fragment))) errors.push(`${file}:${link.line}: missing heading ${link.href}`);
        }
      } catch (error) {
        errors.push(`${file}:${link.line}: invalid link ${link.href}: ${error.message}`);
      }
    }
  }
  return { checked, errors };
}

export function repositoryDocs(root) {
  return execFileSync("git", ["ls-files", "--cached", "--others", "--exclude-standard", "-z"], { cwd: root, encoding: "utf8" })
    .split("\0").filter(file => file.endsWith(".md"));
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  const root = fileURLToPath(new URL("../", import.meta.url));
  const result = checkLinks(root, repositoryDocs(root));
  if (result.errors.length) {
    console.error(result.errors.join("\n"));
    process.exitCode = 1;
  } else {
    console.log(`Documentation: ${result.checked} local targets/headings passed; two documented verbatim snapshots excluded.`);
  }
}
