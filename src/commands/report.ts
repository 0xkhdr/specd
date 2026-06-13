// `specd report <slug> [--format md|html] [--out <path>]` — deterministic snapshot (SPEC §5.2, §11).
import { resolve } from "node:path";
import { requireSpecdRoot } from "../core/paths.js";
import { loadSpec, loadConfig, readArtifact } from "../core/specFiles.js";
import { atomicWrite } from "../core/io.js";
import { stdout } from "../core/output.js";
import { usageError } from "../core/exit.js";
import { renderHtml, renderMarkdown, type ReportData } from "../core/report.js";
import type { Args } from "../cli.js";

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  if (!slug) throw usageError("usage: specd report <slug> [--format md|html] [--out <path>]");
  const { state } = loadSpec(root, slug);
  const cfg = loadConfig(root);

  const format = (typeof args.flags.format === "string" ? args.flags.format : cfg.report.format) as "md" | "html";
  if (format !== "md" && format !== "html") throw usageError("--format must be md or html");

  const data: ReportData = {
    state,
    requirements: readArtifact(root, slug, "requirements.md"),
    design: readArtifact(root, slug, "design.md"),
    tasks: readArtifact(root, slug, "tasks.md"),
    decisions: readArtifact(root, slug, "decisions.md"),
    memory: readArtifact(root, slug, "memory.md"),
    midReqs: readArtifact(root, slug, "mid-requirements.md"),
  };

  const out = format === "html" ? renderHtml(data, cfg.report.autoRefreshSeconds) : renderMarkdown(data);

  if (typeof args.flags.out === "string") {
    const path = resolve(process.cwd(), args.flags.out);
    atomicWrite(path, out);
    console.log(`report: wrote ${format} → ${args.flags.out}`);
  } else {
    stdout.write(out);
  }
  return 0;
}
