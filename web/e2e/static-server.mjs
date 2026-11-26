// Serves ../examples for the end-to-end test: node e2e/static-server.mjs <port>
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { extname, join, normalize } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../examples/", import.meta.url));
const port = Number(process.argv[2] ?? 8098);
const types = { ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8" };

createServer(async (req, res) => {
  let path = decodeURIComponent(new URL(req.url ?? "/", "http://x").pathname);
  if (path === "/") path = "/demo.html";
  const file = normalize(join(root, path));
  if (!file.startsWith(root)) {
    res.writeHead(404).end();
    return;
  }
  try {
    const body = await readFile(file);
    res
      .writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream" })
      .end(body);
  } catch {
    res.writeHead(404).end("not found");
  }
}).listen(port, "127.0.0.1", () => console.log(`examples on http://localhost:${port}`));
