import { createReadStream, statSync } from "node:fs";
import { createServer } from "node:http";
import { extname, join, normalize, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("../dist/", import.meta.url)));
const contentTypes = {
  ".css": "text/css; charset=utf-8",
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml",
  ".woff2": "font/woff2"
};

const server = createServer((request, response) => {
  const pathname = decodeURIComponent(new URL(request.url ?? "/", "http://127.0.0.1").pathname);
  const relative = normalize(pathname).replace(/^([/\\])+/, "");
  let target = resolve(join(root, relative || "index.html"));
  if (target !== root && !target.startsWith(`${root}${sep}`)) {
    response.writeHead(403).end("Forbidden");
    return;
  }
  try {
    if (statSync(target).isDirectory()) target = join(target, "index.html");
    if (!statSync(target).isFile()) throw new Error("not a file");
  } catch {
    target = join(root, "index.html");
  }
  response.setHeader("Content-Type", contentTypes[extname(target)] ?? "application/octet-stream");
  createReadStream(target)
    .on("error", () => response.writeHead(500).end("Read failure"))
    .pipe(response);
});

const shutdown = () => server.close(() => process.exit(0));
process.once("SIGINT", shutdown);
process.once("SIGTERM", shutdown);
server.listen(4173, "127.0.0.1");
