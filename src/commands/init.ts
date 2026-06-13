// `specd init [--force]` — scaffold .specd/ + AGENTS.md (SPEC §5.2). Idempotent.
import { existsSync } from "node:fs";
import { atomicWrite } from "../core/io.js";
import { agentsPath, configPath, specdDir, rolesDir, steeringDir } from "../core/paths.js";
import { readTemplate } from "../core/templates.js";
import type { Args } from "../cli.js";
import * as ui from "../core/ui.js";

const STEERING = ["reasoning.md", "workflow.md", "product.md", "tech.md", "structure.md", "memory.md"];
const ROLES = ["investigator.md", "builder.md", "reviewer.md", "verifier.md"];

export function run(args: Args): number {
  const root = process.cwd();
  const force = args.flags.force === true;
  const written: string[] = [];
  const skipped: string[] = [];

  const place = (dest: string, content: string) => {
    if (existsSync(dest) && !force) { skipped.push(dest); return; }
    atomicWrite(dest, content);
    written.push(dest);
  };

  // ensure .specd exists implicitly via atomicWrite mkdir; place files:
  for (const f of STEERING) place(`${steeringDir(root)}/${f}`, readTemplate(`steering/${f}`));
  for (const f of ROLES) place(`${rolesDir(root)}/${f}`, readTemplate(`roles/${f}`));
  place(configPath(root), readTemplate("config.json"));
  place(agentsPath(root), readTemplate("AGENTS.md"));

  // touch .specd dir marker (in case all files skipped, dir already exists)
  void specdDir(root);

  const rel = (p: string) => p.replace(root + "/", "");
  if (written.length) {
    ui.info(`specd init: wrote ${written.length} file(s):`);
    for (const w of written) ui.info(`  + ${rel(w)}`);
  }
  if (skipped.length) {
    ui.info(`skipped ${skipped.length} existing file(s) (use --force to overwrite):`);
    for (const s of skipped) ui.info(`  · ${rel(s)}`);
  }
  if (!written.length && !skipped.length) ui.info("specd init: nothing to do");
  return 0;
}
