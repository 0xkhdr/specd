// `specd new <slug> [--title "..."]` — create a spec folder with six artifacts + state.json (§5.2).
import { join } from "node:path";
import { atomicWrite } from "../core/io.js";
import { EXIT, usageError, gateError, SpecdError } from "../core/exit.js";
import { requireSpecdRoot, specDir } from "../core/paths.js";
import { ARTIFACTS, specExists } from "../core/specFiles.js";
import { initialState, saveState } from "../core/state.js";
import { applyVars, readTemplate } from "../core/templates.js";
import type { Args } from "../cli.js";

const SLUG_RE = /^[a-z0-9][a-z0-9-]*$/;

const titleCase = (slug: string) =>
  slug.split("-").map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(" ");

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  if (!slug) throw usageError("usage: specd new <slug> [--title \"...\"]");
  if (!SLUG_RE.test(slug)) throw usageError(`invalid slug '${slug}' (must match ^[a-z0-9][a-z0-9-]*$)`);
  if (specExists(root, slug)) throw new SpecdError(EXIT.GATE, `spec '${slug}' already exists`);

  const title = typeof args.flags.title === "string" ? args.flags.title : titleCase(slug);
  const date = new Date().toISOString().slice(0, 10);
  const vars = { TITLE: title, SLUG: slug, DATE: date };

  const dir = specDir(root, slug);
  for (const name of ARTIFACTS) {
    const tmpl = readTemplate(`specStubs/${name}`);
    atomicWrite(join(dir, name), applyVars(tmpl, vars));
  }
  saveState(root, slug, initialState(slug, title));

  console.log(`specd new: created spec '${slug}' (${title})`);
  console.log(`  .specd/specs/${slug}/ — six artifacts + state.json (status: requirements)`);
  console.log("Next: write requirements.md (EARS), then `specd check " + slug + "`.");
  return 0;
}
