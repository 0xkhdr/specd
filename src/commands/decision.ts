// `specd decision <slug> "<text>" [--supersedes <id>]` — append an ADR entry (SPEC §5.2, §7.4).
import { requireSpecdRoot } from "../core/paths.js";
import { artifactPath, requireSpec } from "../core/specFiles.js";
import { appendFile, readOrDefault } from "../core/io.js";
import { withSpecLock } from "../core/lock.js";
import { usageError } from "../core/exit.js";
import { stripHtmlComments } from "../core/md.js";
import type { Args } from "../cli.js";

const nextAdrNumber = (text: string): number => {
  const nums = [...stripHtmlComments(text).matchAll(/^##\s+ADR-(\d+)/gm)].map((m) => parseInt(m[1], 10));
  return (nums.length ? Math.max(...nums) : 0) + 1;
};

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  const text = args.pos[1];
  if (!slug || !text) throw usageError('usage: specd decision <slug> "<decision text>" [--supersedes <ADR-id>]');
  requireSpec(root, slug);

  const path = artifactPath(root, slug, "decisions.md");
  const supersedes = typeof args.flags.supersedes === "string" ? args.flags.supersedes : "—";
  const date = new Date().toISOString().slice(0, 10);

  // Lock the read-number→append so two concurrent decisions can't mint the same ADR id.
  return withSpecLock(root, slug, () => {
    const id = `ADR-${String(nextAdrNumber(readOrDefault(path, ""))).padStart(3, "0")}`;
    const entry = [
      "",
      `## ${id} — ${text} · ${date}`,
      `**Context:** TODO`,
      `**Decision:** ${text}`,
      `**Consequences:** TODO`,
      `**Supersedes:** ${supersedes}`,
      "",
    ].join("\n");
    appendFile(path, entry);
    console.log(`decision: appended ${id} to decisions.md`);
    return 0;
  });
}
