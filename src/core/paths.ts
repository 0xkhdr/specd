// Locate the .specd harness root by walking up from cwd (SPEC §5.1).
import { existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { notFoundError } from "./exit.js";

/** Walk up from `start` to find the nearest ancestor containing `.specd/`. Returns null if none. */
export function findSpecdRoot(start: string = process.cwd()): string | null {
  let dir = resolve(start);
  // eslint-disable-next-line no-constant-condition
  while (true) {
    if (existsSync(join(dir, ".specd"))) return dir;
    const parent = dirname(dir);
    if (parent === dir) return null; // hit fs root
    dir = parent;
  }
}

/** Like findSpecdRoot but throws NotFound (exit 3) when absent. */
export function requireSpecdRoot(start: string = process.cwd()): string {
  const root = findSpecdRoot(start);
  if (!root) {
    throw notFoundError("no .specd/ found in this directory or any parent. Run `specd init` first.");
  }
  return root;
}

export const specdDir = (root: string) => join(root, ".specd");
export const steeringDir = (root: string) => join(root, ".specd", "steering");
export const rolesDir = (root: string) => join(root, ".specd", "roles");
export const specsDir = (root: string) => join(root, ".specd", "specs");
export const specDir = (root: string, slug: string) => join(root, ".specd", "specs", slug);
export const configPath = (root: string) => join(root, ".specd", "config.json");
export const agentsPath = (root: string) => join(root, "AGENTS.md");

/** Absolute path to the shipped templates directory (dist/templates at runtime). */
export function templatesDir(): string {
  const here = dirname(fileURLToPath(import.meta.url)); // dist/core or src/core
  // dist layout: dist/core/paths.js -> dist/templates ; src layout: src/core/paths.ts -> src/templates
  return join(dirname(here), "templates");
}
