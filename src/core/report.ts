// Deterministic report assembly (SPEC §5.2, §11). No LLM, no narrative — assembles artifacts.
import { counts, waveGraph } from "./render.js";
import type { State, SpecStatus } from "./state.js";

export interface Badge { label: string; color: string; }

export function badge(status: SpecStatus): Badge {
  switch (status) {
    case "requirements":
    case "design":
    case "tasks": return { label: "Planning", color: "#a371f7" };
    case "executing": return { label: "Implementing", color: "#d29922" };
    case "verifying": return { label: "Verifying", color: "#58a6ff" };
    case "complete": return { label: "Complete", color: "#3fb950" };
    case "blocked": return { label: "Blocked", color: "#f85149" };
  }
}

export interface ReportData {
  state: State;
  requirements: string | null;
  design: string | null;
  tasks: string | null;
  decisions: string | null;
  memory: string | null;
  midReqs: string | null;
}

/** Extract the body of an H2 section by (case-insensitive) heading prefix. */
export function extractSection(md: string | null, heading: string): string | null {
  if (!md) return null;
  const lines = md.split("\n");
  const start = lines.findIndex((l) => new RegExp(`^##\\s+${heading}`, "i").test(l));
  if (start === -1) return null;
  const body: string[] = [];
  for (let i = start + 1; i < lines.length; i++) {
    if (/^##\s+/.test(lines[i])) break;
    body.push(lines[i]);
  }
  return body.join("\n").trim() || null;
}

function execSummary(d: ReportData): string {
  return (
    extractSection(d.requirements, "Introduction") ??
    extractSection(d.design, "Overview") ??
    "_No summary provided._"
  );
}

function progressOverview(state: State): string {
  const c = counts(state);
  const cards = `**${c.complete}** complete · **${c.running}** running · **${c.pending}** pending · **${c.blocked}** blocked · **${c.total}** total`;
  return `${cards}\n\n\`\`\`\n${waveGraph(state)}\n\`\`\``;
}

interface Section { icon: string; title: string; body: string; }

function sections(d: ReportData): Section[] {
  const s: Section[] = [
    { icon: "📝", title: "Executive Summary", body: execSummary(d) },
    { icon: "📊", title: "Progress Overview", body: progressOverview(d.state) },
    { icon: "📋", title: "Requirements", body: d.requirements ?? "_None._" },
    { icon: "🗺️", title: "Plan / Design", body: d.design ?? "_None._" },
    { icon: "✅", title: "Tasks", body: d.tasks ?? "_None._" },
    { icon: "🔄", title: "Mid-Requirements", body: d.midReqs ?? "_None._" },
    { icon: "🧠", title: "Build Knowledge", body: d.memory ?? "_None._" },
    { icon: "📓", title: "Decisions", body: d.decisions ?? "_None._" },
  ];
  const acc = d.state.acceptance ?? {};
  const accKeys = Object.keys(acc).sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
  if (accKeys.length) {
    const body = accKeys
      .map((k) => `- ${acc[k].status === "pass" ? "✅" : "❌"} **${k}** _(req ${acc[k].requirement})_ — ${acc[k].evidence}`)
      .join("\n");
    s.push({ icon: "🧪", title: "Acceptance Criteria", body });
  }
  if (d.state.blockers.length) {
    const body = d.state.blockers.map((b) => `- **${b.task}** — ${b.reason} _(since ${b.since})_`).join("\n");
    s.push({ icon: "🚧", title: "Blockers", body });
  }
  return s;
}

export function renderMarkdown(d: ReportData): string {
  const b = badge(d.state.status);
  const out: string[] = [];
  out.push(`# ${d.state.title} — [${b.label}]`);
  out.push("");
  out.push(`> Spec: \`${d.state.spec}\` · Status: **${d.state.status}** · Phase: **${d.state.phase}** · Turn: ${d.state.turn}`);
  out.push("");
  for (const sec of sections(d)) {
    out.push(`## ${sec.icon} ${sec.title}`);
    out.push("");
    out.push(sec.body);
    out.push("");
  }
  return out.join("\n").trimEnd() + "\n";
}

const esc = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

export function renderHtml(d: ReportData, autoRefreshSeconds = 0): string {
  const b = badge(d.state.status);
  const refresh = autoRefreshSeconds > 0 ? `\n  <meta http-equiv="refresh" content="${autoRefreshSeconds}">` : "";
  const secHtml = sections(d)
    .map(
      (sec) =>
        `  <section>\n    <h2>${sec.icon} ${esc(sec.title)}</h2>\n    <pre>${esc(sec.body)}</pre>\n  </section>`,
    )
    .join("\n");
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">${refresh}
  <title>${esc(d.state.title)} — ${b.label}</title>
  <style>
    body { font: 15px/1.55 system-ui, sans-serif; max-width: 920px; margin: 2rem auto; padding: 0 1rem; color: #c9d1d9; background: #0d1117; }
    h1 { font-size: 1.6rem; }
    h2 { border-bottom: 1px solid #30363d; padding-bottom: .3rem; margin-top: 2rem; }
    .badge { display: inline-block; padding: .15rem .6rem; border-radius: 1rem; color: #fff; font-size: .85rem; background: ${b.color}; }
    .meta { color: #8b949e; font-size: .9rem; }
    pre { white-space: pre-wrap; background: #161b22; padding: 1rem; border-radius: 6px; overflow-x: auto; }
    section { margin-bottom: 1rem; }
  </style>
</head>
<body>
  <h1>${esc(d.state.title)} <span class="badge">${b.label}</span></h1>
  <p class="meta">Spec: <code>${esc(d.state.spec)}</code> · Status: ${esc(d.state.status)} · Phase: ${esc(d.state.phase)} · Turn: ${d.state.turn}</p>
${secHtml}
</body>
</html>
`;
}
