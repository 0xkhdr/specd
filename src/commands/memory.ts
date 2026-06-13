// `specd memory <slug> add|promote ...` — source-attributed learnings (SPEC §5.2, §7.5).
import { join } from "node:path";
import { requireSpecdRoot, steeringDir } from "../core/paths.js";
import { artifactPath, readArtifact, requireSpec, listSpecs, loadConfig } from "../core/specFiles.js";
import { appendFile, readOrDefault } from "../core/io.js";
import { withSpecLock } from "../core/lock.js";
import { usageError, gateError } from "../core/exit.js";
import { stripHtmlComments } from "../core/md.js";
import type { Args } from "../cli.js";

const CRITS = new Set(["minor", "important", "critical"]);

/** Extract the `## <key>` section block from a memory doc, or null. */
function extractBlock(text: string, key: string): string | null {
  const lines = stripHtmlComments(text).split("\n");
  const start = lines.findIndex((l) => l.trim() === `## ${key}`);
  if (start === -1) return null;
  const body: string[] = [lines[start]];
  for (let i = start + 1; i < lines.length; i++) {
    if (/^##\s+/.test(lines[i])) break;
    body.push(lines[i]);
  }
  return body.join("\n").trimEnd();
}

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  const sub = args.pos[1];
  if (!slug || !sub) throw usageError("usage: specd memory <slug> <add|promote> [flags]");
  requireSpec(root, slug);

  const memPath = artifactPath(root, slug, "memory.md");

  if (sub === "add") {
    const key = args.flags.key;
    const pattern = args.flags.pattern;
    const body = args.flags.body;
    const source = args.flags.source;
    const crit = args.flags.criticality;
    if (typeof key !== "string" || typeof pattern !== "string" || typeof body !== "string" || typeof source !== "string") {
      throw usageError('memory add requires --key --pattern "<..>" --body "<..>" --source "<..>" --criticality <c>');
    }
    if (typeof crit !== "string" || !CRITS.has(crit)) {
      throw usageError("--criticality must be one of: minor, important, critical");
    }
    const related = typeof args.flags.related === "string"
      ? args.flags.related.split(",").map((s) => `[[${s.trim()}]]`).join(", ")
      : "—";
    const entry = [
      "",
      `## ${key}`,
      `**Pattern:** ${pattern}`,
      `**Detail:** ${body}`,
      `**Source:** ${source}`,
      `**Criticality:** ${crit}`,
      `**Related:** ${related}`,
      "",
    ].join("\n");
    return withSpecLock(root, slug, () => {
      appendFile(memPath, entry);
      console.log(`memory: added '${key}' to ${slug}/memory.md`);
      return 0;
    });
  }

  if (sub === "promote") {
    const key = args.flags.key;
    if (typeof key !== "string") throw usageError("memory promote requires --key <slug>");
    return withSpecLock(root, slug, () => {
    const block = extractBlock(readOrDefault(memPath, ""), key);
    if (!block) throw gateError(`memory: key '${key}' not found in ${slug}/memory.md`);

    // Promotion threshold: only promote a pattern that has recurred across ≥threshold specs, so
    // one-off learnings don't pollute global steering. `--force` overrides (e.g. a known-critical
    // pattern seen once). A threshold ≤1 disables the gate.
    const threshold = loadConfig(root).promotionThreshold;
    const occurrences = listSpecs(root).filter((s) => extractBlock(readArtifact(root, s, "memory.md") ?? "", key) !== null).length;
    if (occurrences < threshold && args.flags.force !== true) {
      throw gateError(`memory: pattern '${key}' seen in ${occurrences} spec(s); promotion threshold is ${threshold}. Re-run with --force to promote anyway.`);
    }

    const date = new Date().toISOString().slice(0, 10);
    const promoted = `\n${block}\n**Promoted:** from spec '${slug}' on ${date} (seen in ${occurrences} spec(s))\n`;
    appendFile(join(steeringDir(root), "memory.md"), promoted);
    console.log(`memory: promoted '${key}' from ${slug} to steering/memory.md (seen in ${occurrences} spec(s), threshold ${threshold})`);
    return 0;
    });
  }

  throw usageError(`unknown memory subcommand '${sub}' (expected add|promote)`);
}
