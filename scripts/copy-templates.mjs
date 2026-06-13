// Copies src/templates -> dist/templates after tsc compile (templates are shipped verbatim).
import { cpSync, existsSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const src = join(root, "src", "templates");
const dest = join(root, "dist", "templates");

if (!existsSync(src)) {
  console.error("no src/templates to copy");
  process.exit(1);
}
mkdirSync(dest, { recursive: true });
cpSync(src, dest, { recursive: true });
console.log("copied templates -> dist/templates");
